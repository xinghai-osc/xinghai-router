package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Service) createRewardClaimTx(ctx context.Context, tx pgx.Tx, d rewardRiskDecision, userID, originID, source, sourceID, amount string) (string, error) {
	status := "pending"
	id, err := randomID()
	if err != nil {
		return "", err
	}
	var savedID string
	err = tx.QueryRow(ctx, `insert into reward_claims(id,source,source_id,user_id,origin_user_id,amount,status,risk_event_id,created_at,updated_at) values($1,$2,$3,$4,$5,$6,'pending',$7,clock_timestamp(),clock_timestamp()) on conflict(source,source_id,user_id) do nothing returning id::text`, id, source, sourceID, userID, originID, amount, d.ID).Scan(&savedID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `select status from reward_claims where source=$1 and source_id=$2 and user_id=$3`, source, sourceID, userID).Scan(&status)
		return status, err
	}
	if err != nil {
		return "", err
	}
	if d.Action == "allow" {
		err = s.creditRewardClaimTx(ctx, tx, savedID)
		if err == nil {
			status = "credited"
		}
	}
	return status, err
}

func (s *Service) creditRewardClaimTx(ctx context.Context, tx pgx.Tx, id string) error {
	var userID, originID, source, sourceID, amount, status string
	var positive bool
	if err := tx.QueryRow(ctx, `select user_id::text,origin_user_id::text,source,source_id,amount::text,amount>0,status from reward_claims where id=$1 for update`, id).Scan(&userID, &originID, &source, &sourceID, &amount, &positive, &status); err != nil {
		return err
	}
	if status == "credited" {
		return nil
	}
	if status != "pending" {
		return errors.New("reward has already been rejected or withdrawn")
	}
	var eligible bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from users where id=$1 and enabled) and exists(select 1 from users where id=$2 and enabled)`, userID, originID).Scan(&eligible); err != nil {
		return err
	}
	if !eligible {
		return errRiskAccountRestricted
	}
	if source == "invitation" {
		if err := tx.QueryRow(ctx, `select exists(select 1 from invitations i join users u on u.id=i.inviter_id where i.id=$1::uuid and u.enabled)`, sourceID).Scan(&eligible); err != nil {
			return err
		}
		if !eligible {
			return errRiskAccountRestricted
		}
	}
	if positive {
		if _, err := tx.Exec(ctx, `insert into user_wallets(user_id) values($1) on conflict(user_id) do nothing`, userID); err != nil {
			return err
		}
		ledgerID, err := randomID()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `with credited as (
   update user_wallets set balance=balance+$2::numeric,updated_at=clock_timestamp() where user_id=$1 returning balance
  ) insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note,reward_claim_id)
  select $3,$1,$2::numeric,balance,$4,$5,$6,$7 from credited`, userID, amount, ledgerID, source, "reward:"+id, "Approved "+source+" reward", id)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `update reward_claims set status='credited',updated_at=clock_timestamp() where id=$1`, id); err != nil {
		return err
	}
	if source == "checkin" {
		if _, err := tx.Exec(ctx, `update user_checkins set reward_status='credited' where reward_source_id=$1::uuid and user_id=$2`, sourceID, userID); err != nil {
			return err
		}
	}
	return nil
}
