package app

import "context"

func (s *Service) walletLedger(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx, `with charge_rows as (
			select wl.id,wl.user_id,wl.amount,wl.balance_after,wl.created_at,
				coalesce(nullif(ws.status,''),nullif(wl.settlement_status,'not_applicable'),'not_applicable') as settlement_status,
				coalesce(ws.business_date,wl.settlement_date,(wl.created_at at time zone 'UTC')::date) as business_date,
				coalesce(ws.settled_at,wl.settled_at) as settled_at,
				coalesce(ws.error,'') as settlement_error
			from wallet_ledger wl
			left join wallet_settlements ws on ws.ledger_id=wl.id
			where wl.user_id=$1 and wl.kind='charge'
		), charge_groups as (
			select user_id,business_date,
				sum(amount)::text as amount,
				(array_agg(balance_after::text order by created_at desc,id desc))[1] as balance_after,
				max(created_at) as created_at,
				case max(case settlement_status when 'failed' then 5 when 'processing' then 4 when 'pending' then 3 when 'settled' then 2 else 1 end)
					when 5 then 'failed' when 4 then 'processing' when 3 then 'pending' when 2 then 'settled' else 'not_applicable' end as settlement_status,
				max(settled_at) as settled_at,
				coalesce(string_agg(distinct nullif(settlement_error,''),'; '),'') as settlement_error,
				count(*)::bigint as call_count
			from charge_rows
			group by user_id,business_date
		)
		select ledger.id,ledger.amount,ledger.balance_after,ledger.kind,ledger.request_id,ledger.note,ledger.created_at,ledger.settlement_status,ledger.settlement_date,ledger.settled_at,ledger.settlement_error,ledger.daily,ledger.business_date,ledger.call_count
		from (
			select concat('daily-charge:',user_id::text,':',business_date::text) as id,amount,balance_after,'charge' as kind,null::text as request_id,null::text as note,created_at,settlement_status,business_date::text as settlement_date,settled_at,settlement_error,true as daily,business_date::text as business_date,call_count
			from charge_groups
			union all
			select wl.id::text,wl.amount::text,wl.balance_after::text,wl.kind,wl.request_id,wl.note,wl.created_at,wl.settlement_status,wl.settlement_date::text,wl.settled_at,coalesce(ws.error,''),false,null::text,0::bigint
			from wallet_ledger wl
			left join wallet_settlements ws on ws.ledger_id=wl.id
			where wl.user_id=$1 and wl.kind not in ('charge','reservation','release')
		) ledger
		order by ledger.created_at desc,ledger.id desc
		limit 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	data := make([]map[string]any, 0)
	for rows.Next() {
		var id, amount, balanceAfter, kind, settlementStatus string
		var requestID, note, createdAt, settlementDate, settledAt, settlementError, businessDate any
		var daily bool
		var callCount int64
		if err := rows.Scan(&id, &amount, &balanceAfter, &kind, &requestID, &note, &createdAt, &settlementStatus, &settlementDate, &settledAt, &settlementError, &daily, &businessDate, &callCount); err != nil {
			return nil, err
		}
		data = append(data, map[string]any{
			"id": id, "amount": amount, "balance_after": balanceAfter, "kind": kind,
			"request_id": requestID, "note": note, "created_at": createdAt,
			"settlement_status": settlementStatus, "settlement_date": settlementDate,
			"settled_at": settledAt, "settlement_error": settlementError,
			"daily": daily, "business_date": businessDate, "call_count": callCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return data, nil
}
