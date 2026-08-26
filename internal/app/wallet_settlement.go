package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

const walletSettlementBatchSize = 100

func walletBusinessDate(now time.Time) time.Time {
	y, m, d := now.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) startWalletSettlementScheduler(ctx context.Context) {
	go func() {
		if err := s.settleWalletDay(ctx, walletBusinessDate(time.Now()).AddDate(0, 0, -1)); err != nil {
			log.Printf("wallet settlement failed: %v", err)
		}
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if err := s.settleWalletDay(ctx, walletBusinessDate(now).AddDate(0, 0, -1)); err != nil {
					log.Printf("wallet settlement failed: %v", err)
				}
			}
		}
	}()
}

func (s *Service) settleWalletDay(ctx context.Context, businessDate time.Time) error {
	batchID, err := randomID()
	if err != nil {
		return fmt.Errorf("create settlement batch id: %w", err)
	}
	var batchStatus string
	err = s.db.QueryRow(ctx, `insert into wallet_settlement_batches(id,business_date,status)
		values($1,$2,'processing')
		on conflict(business_date) do update set id=wallet_settlement_batches.id
		returning id,status`, batchID, businessDate).Scan(&batchID, &batchStatus)
	if err != nil {
		return fmt.Errorf("create settlement batch: %w", err)
	}
	if batchStatus == "settled" {
		return nil
	}
	if _, err = s.db.Exec(ctx, `update wallet_settlement_batches set status='processing',started_at=coalesce(started_at,now()),finished_at=null,error='' where id=$1`, batchID); err != nil {
		return fmt.Errorf("start settlement batch: %w", err)
	}
	if _, err = s.db.Exec(ctx, `update wallet_settlements set status='pending',batch_id=null,error='',updated_at=now()
		where business_date=$1 and status in ('processing','failed')`, businessDate); err != nil {
		return fmt.Errorf("recover settlement items: %w", err)
	}
	if _, err = s.db.Exec(ctx, `update wallet_ledger wl set settlement_status='pending',settlement_batch_id=null
		from wallet_settlements ws where ws.ledger_id=wl.id and ws.business_date=$1 and ws.status='pending'`, businessDate); err != nil {
		return fmt.Errorf("recover ledger entries: %w", err)
	}
	for {
		processed, err := s.settleWalletBatch(ctx, batchID, businessDate)
		if err != nil {
			_, _ = s.db.Exec(ctx, `update wallet_settlement_batches set status='failed',error=$1,finished_at=now() where id=$2`, err.Error(), batchID)
			return err
		}
		if processed == 0 {
			break
		}
	}
	if _, err = s.db.Exec(ctx, `update wallet_settlement_batches set status='settled',finished_at=now(),error='' where id=$1`, batchID); err != nil {
		return fmt.Errorf("finish settlement batch: %w", err)
	}
	return nil
}

func (s *Service) settleWalletBatch(ctx context.Context, batchID string, businessDate time.Time) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin settlement batch: %w", err)
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `select ws.id,ws.ledger_id,ws.user_id,ws.amount,ws.request_id
		from wallet_settlements ws
		where ws.business_date=$1 and ws.status='pending'
		order by ws.created_at,ws.id
		for update skip locked limit $2`, businessDate, walletSettlementBatchSize)
	if err != nil {
		return 0, fmt.Errorf("claim settlement items: %w", err)
	}
	type item struct {
		id, ledgerID, userID, requestID string
		amount                          float64
	}
	items := make([]item, 0, walletSettlementBatchSize)
	for rows.Next() {
		var current item
		if err := rows.Scan(&current.id, &current.ledgerID, &current.userID, &current.amount, &current.requestID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan settlement item: %w", err)
		}
		items = append(items, current)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("read settlement items: %w", err)
	}
	rows.Close()
	for _, current := range items {
		if _, err := tx.Exec(ctx, `update wallet_settlements set status='processing',batch_id=$1,updated_at=now() where id=$2`, batchID, current.id); err != nil {
			return 0, fmt.Errorf("mark settlement item: %w", err)
		}
		var balanceAfter float64
		if err := tx.QueryRow(ctx, `update user_wallets set balance=balance-$1,reserved=greatest(0,reserved-$1),updated_at=now() where user_id=$2 and balance >= $1 returning balance`, current.amount, current.userID).Scan(&balanceAfter); err != nil {
			message := "insufficient balance at daily settlement"
			if _, updateErr := tx.Exec(ctx, `update wallet_settlements set status='failed',error=$1,updated_at=now() where id=$2`, message, current.id); updateErr != nil {
				return 0, fmt.Errorf("mark failed settlement: %w", updateErr)
			}
			continue
		}
		if _, err := tx.Exec(ctx, `update wallet_ledger set balance_after=$1,settlement_status='settled',settled_at=now(),settlement_batch_id=$2 where id=$3`, balanceAfter, batchID, current.ledgerID); err != nil {
			return 0, fmt.Errorf("settle ledger entry: %w", err)
		}
		if _, err := tx.Exec(ctx, `update wallet_settlements set status='settled',settled_at=now(),updated_at=now() where id=$1`, current.id); err != nil {
			return 0, fmt.Errorf("finish settlement item: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit settlement batch: %w", err)
	}
	return len(items), nil
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
