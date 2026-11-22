//go:build integration

package app

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestIntegrationWalletSettlementCatchup(t *testing.T) {
	db, _ := integrationPool(t)
	defer db.Close()
	resetIntegrationDatabase(t, db)
	ctx := context.Background()
	s := &Service{db: db}
	funded, _ := integrationUser(t, db, "funded@example.com", 200)
	short, _ := integrationUser(t, db, "short@example.com", 1)
	now := time.Date(2026, 2, 17, 0, 15, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -5)
	other := now.AddDate(0, 0, -2)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	charge := func(user, prefix string, day time.Time, count int) {
		t.Helper()
		exec(`with charged as (insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,note,settlement_status,settlement_date)
			select gen_random_uuid(),$1,-1,0,'charge',$2::text||i,'test','pending',$3 from generate_series(1,$4::int) i
			returning id,user_id,request_id,settlement_date)
			insert into wallet_settlements(id,ledger_id,user_id,request_id,business_date,amount)
			select gen_random_uuid(),id,user_id,request_id,settlement_date,1 from charged`, user, prefix, day, count)
	}
	charge(funded, "backlog-", old, 101)
	charge(funded, "backlog-other-", other, 1)
	charge(funded, "today-", now, 1)
	charge(short, "short-", old, 1)
	exec(`update user_wallets set balance=0,reserved=1 where user_id=$1`, short)
	exec(`update user_wallets set reserved=110 where user_id=$1`, funded)
	exec(`insert into wallet_ledger(id,user_id,amount,balance_after,kind,request_id,created_at) values(gen_random_uuid(),$1,-7,200,'reservation','long-stream',$2)`, funded, old)
	exec(`insert into wallet_settlement_batches(id,business_date,status) values(gen_random_uuid(),$1,'settled')`, old)
	exec(`update wallet_settlements set status='processing' where request_id='backlog-1'`)
	exec(`update wallet_settlements set status='failed' where request_id='backlog-2'`)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.settleDueWalletDays(ctx, now); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	check := func(sql string, args ...any) {
		t.Helper()
		var ok bool
		if err := db.QueryRow(ctx, sql, args...).Scan(&ok); err != nil || !ok {
			t.Fatalf("assertion failed for %s: ok=%v err=%v", sql, ok, err)
		}
	}
	check(`select balance=98 and reserved=8 from user_wallets where user_id=$1`, funded)
	check(`select balance=0 and reserved=1 from user_wallets where user_id=$1`, short)
	check(`select count(*)=102 from wallet_settlements where status='settled'`)
	check(`select status='failed' from wallet_settlement_batches where business_date=$1`, old)
	check(`select status='settled' from wallet_settlement_batches where business_date=$1`, other)
	check(`select ws.status='failed' and wl.settlement_status='failed' and ws.retry_after > $1 from wallet_settlements ws join wallet_ledger wl on wl.id=ws.ledger_id where ws.request_id='short-1'`, now)
	check(`select status='pending' from wallet_settlements where request_id='today-1'`)
	check(`select count(*)=0 from wallet_settlement_batches where business_date=$1`, now)
	exec(`update user_wallets set balance=10 where user_id=$1`, short)
	if err := s.settleDueWalletDays(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	check(`select balance=10 and reserved=1 from user_wallets where user_id=$1`, short)
	for i := 0; i < 2; i++ {
		if err := s.settleDueWalletDays(ctx, now.Add(walletSettlementInterval)); err != nil {
			t.Fatal(err)
		}
	}
	check(`select balance=9 and reserved=0 from user_wallets where user_id=$1`, short)
	check(`select status='settled' from wallet_settlement_batches where business_date=$1`, old)
	check(`select ws.status='settled' and wl.settlement_status='settled' and ws.retry_after is null from wallet_settlements ws join wallet_ledger wl on wl.id=ws.ledger_id where ws.request_id='short-1'`)
	charge(funded, "late-", old, 1)
	exec(`update user_wallets set reserved=reserved+1 where user_id=$1`, funded)
	if err := s.settleDueWalletDays(ctx, now.Add(walletSettlementInterval)); err != nil {
		t.Fatal(err)
	}
	check(`select balance=97 and reserved=8 from user_wallets where user_id=$1`, funded)
	check(`select count(*)=104 from wallet_settlements where status='settled'`)
}
