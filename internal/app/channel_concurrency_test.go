package app

import (
	"context"
	"encoding/json"
	"testing"
)

func TestChannelMaxConcurrencyNormalization(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		want     int
		supplied bool
		wantErr  bool
	}{
		{name: "absent", raw: "", want: 0, supplied: false},
		{name: "null", raw: "null", want: 0, supplied: true},
		{name: "zero means unlimited", raw: "0", want: 0, supplied: true},
		{name: "value", raw: "17", want: 17, supplied: true},
		{name: "upper bound", raw: "10000", want: 10000, supplied: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "above upper bound", raw: "10001", wantErr: true},
		{name: "fraction", raw: "1.5", wantErr: true},
		{name: "quoted number", raw: `"5"`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value, supplied, err := channelMaxConcurrency(json.RawMessage(tc.raw))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if value != tc.want || supplied != tc.supplied {
				t.Fatalf("value = %d supplied = %v, want %d %v", value, supplied, tc.want, tc.supplied)
			}
		})
	}
}

func TestAcquireChannelConcurrencyInProcess(t *testing.T) {
	ctx := context.Background()
	s := &Service{channelLimiter: NewGroupLimiter()}
	limited := channel{id: 7, maxConcurrency: 1}

	if _, release, busy, err := s.acquireChannelConcurrency(ctx, channel{id: 8}); err != nil || busy {
		t.Fatalf("unlimited channel: busy=%v err=%v", busy, err)
	} else {
		release()
	}

	_, release, busy, err := s.acquireChannelConcurrency(ctx, limited)
	if err != nil || busy {
		t.Fatalf("first acquire: busy=%v err=%v", busy, err)
	}
	if _, _, busy, err := s.acquireChannelConcurrency(ctx, limited); err != nil || !busy {
		t.Fatalf("second acquire must be refused: busy=%v err=%v", busy, err)
	}

	other := channel{id: 9, maxConcurrency: 1}
	_, releaseOther, busy, err := s.acquireChannelConcurrency(ctx, other)
	if err != nil || busy {
		t.Fatalf("separate channel shares no counter: busy=%v err=%v", busy, err)
	}
	releaseOther()

	release()
	release()
	if _, releaseAgain, busy, err := s.acquireChannelConcurrency(ctx, limited); err != nil || busy {
		t.Fatalf("released slot was not reusable: busy=%v err=%v", busy, err)
	} else {
		releaseAgain()
	}
}

func TestAcquireChannelConcurrencyClusterRequiresLeaseBackend(t *testing.T) {
	s := &Service{cfg: Config{DeploymentMode: "cluster"}}
	if _, _, _, err := s.acquireChannelConcurrency(context.Background(), channel{id: 1, maxConcurrency: 1}); err == nil {
		t.Fatal("cluster mode without a lease manager must fail instead of admitting")
	}
}
