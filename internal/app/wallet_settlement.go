package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

const walletSettlementBatchSize = 100
const walletSettlementInterval = 10 * time.Minute

func walletBusinessDate(now time.Time) time.Time {
	y, m, d := now.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) startWalletSettlementScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(walletSettlementInterval)
		defer ticker.Stop()
		for {
			if err := s.settleDueWalletDays(ctx, time.Now()); err != nil && ctx.Err() == nil {
				log.Printf("wallet settlement failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) settleDueWalletDays(ctx context.Context, now time.Time) error {
	rows, err := s.db.Query(ctx, `select business_date from wallet_settlements
		where business_date < $1 and status <> 'settled' and (retry_after is null or retry_after <= $2)
		union select business_date from wallet_settlement_batches where business_date < $1 and status = 'processing'
		order by business_date`, walletBusinessDate(now), now)
	if err != nil {
		return fmt.Errorf("list due settlement days: %w", err)
	}
	days, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return fmt.Errorf("read due settlement days: %w", err)
	}
	var dayErrors []error
	for _, day := range days {
		for {
			done, err := s.settleWalletBatch(ctx, day, now)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				dayErrors = append(dayErrors, fmt.Errorf("settle %s: %w", day.Format(time.DateOnly), err))
				break
			}
			if done {
				break
			}
		}
	}
	return errors.Join(dayErrors...)
}

func (s *Service) settleWalletBatch(ctx context.Context, day, now time.Time) (bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var batchID string
	err = tx.QueryRow(ctx, `insert into wallet_settlement_batches(id,business_date,status)
		values(gen_random_uuid(),$1,'processing') on conflict (business_date) do update
		set status='processing',finished_at=null,error='' returning id::text`, day).Scan(&batchID)
	if err != nil {
		return false, fmt.Errorf("lock settlement batch: %w", err)
	}
	rows, err := tx.Query(ctx, `select id::text,ledger_id::text,user_id::text,amount::text from wallet_settlements
		where business_date=$1 and status <> 'settled' and (retry_after is null or retry_after <= $2)
		order by created_at,id limit $3 for update`, day, now, walletSettlementBatchSize)
	if err != nil {
		return false, fmt.Errorf("claim settlement items: %w", err)
	}
	type item struct{ id, ledgerID, userID, amount string }
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (item, error) {
		var v item
		err := row.Scan(&v.id, &v.ledgerID, &v.userID, &v.amount)
		return v, err
	})
	if err != nil {
		return false, fmt.Errorf("read settlement items: %w", err)
	}
	for _, v := range items {
		var balance string
		err = tx.QueryRow(ctx, `update user_wallets set balance=balance-$1,reserved=reserved-$1,updated_at=now()
			where user_id=$2 and balance >= $1 and reserved >= $1 returning balance::text`, v.amount, v.userID).Scan(&balance)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := tx.Exec(ctx, `update wallet_settlements set status='failed',batch_id=$2,error='insufficient balance or reservation at daily settlement',
				retry_after=$3,updated_at=now() where id=$1`, v.id, batchID, now.Add(walletSettlementInterval)); err != nil {
				return false, fmt.Errorf("defer unfunded settlement: %w", err)
			}
			if _, err := tx.Exec(ctx, `update wallet_ledger set settlement_status='failed',settlement_batch_id=$2 where id=$1`, v.ledgerID, batchID); err != nil {
				return false, fmt.Errorf("mark failed ledger entry: %w", err)
			}
			continue
		}
		if err != nil {
			return false, fmt.Errorf("debit wallet: %w", err)
		}
		if _, err := tx.Exec(ctx, `update wallet_ledger set balance_after=$1,settlement_status='settled',settled_at=now(),settlement_batch_id=$2 where id=$3`, balance, batchID, v.ledgerID); err != nil {
			return false, fmt.Errorf("settle ledger entry: %w", err)
		}
		if _, err := tx.Exec(ctx, `update wallet_settlements set status='settled',batch_id=$2,settled_at=now(),updated_at=now(),retry_after=null,error='' where id=$1`, v.id, batchID); err != nil {
			return false, fmt.Errorf("finish settlement item: %w", err)
		}
	}
	done := len(items) < walletSettlementBatchSize
	if done {
		_, err := tx.Exec(ctx, `update wallet_settlement_batches b set
			status=case when remaining then 'failed' else 'settled' end,
			error=case when remaining then 'settlement items require retry' else '' end,finished_at=now()
			from (select exists(select 1 from wallet_settlements where business_date=$1 and status <> 'settled') as remaining) s where b.id=$2`, day, batchID)
		if err != nil {
			return false, fmt.Errorf("finish settlement batch: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit settlement batch: %w", err)
	}
	return done, nil
}

func (s *Service) createWalletSettlementTx(ctx context.Context, tx pgx.Tx, ledgerID, userID, requestID string, amount float64, createdAt time.Time) error {
	settlementID, err := randomID()
	if err != nil {
		return fmt.Errorf("create settlement id: %w", err)
	}
	businessDate := walletBusinessDate(createdAt)
	_, err = tx.Exec(ctx, `insert into wallet_settlements(id,ledger_id,user_id,request_id,business_date,amount)
		values($1,$2,$3,$4,$5,$6)
		on conflict(request_id) do nothing`, settlementID, ledgerID, userID, requestID, businessDate, amount)
	return err
}
