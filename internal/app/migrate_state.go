package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	xinghaimigrate "github.com/xinghai-osc/xinghai-router/internal/migrate"
)

type migrationStatus struct {
	mu         *sync.Mutex
	Status     string    `json:"status"`
	Step       string    `json:"step"`
	Current    int       `json:"current"`
	Total      int       `json:"total"`
	Detail     string    `json:"detail,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

func (s *Service) startMigration(sourceDSN, sourceDriver string) bool {
	s.migration.mu.Lock()
	defer s.migration.mu.Unlock()
	if s.migration.Status == "running" {
		return false
	}
	s.migration = migrationStatus{mu: &sync.Mutex{}, Status: "running", Step: "connect", Current: 0, Total: 0, Detail: "Connecting to source and target databases", StartedAt: time.Now()}
	ctx, cancel := context.WithCancel(context.Background())
	s.migrationCancel = cancel
	go s.runMigrationAsync(ctx, sourceDSN, sourceDriver)
	return true
}

func (s *Service) migrationSnapshot() migrationStatus {
	s.migration.mu.Lock()
	cp := s.migration
	s.migration.mu.Unlock()
	return cp
}

func (s *Service) runMigrationAsync(ctx context.Context, sourceDSN, sourceDriver string) {
	err := xinghaimigrate.Run(ctx, sourceDSN, sourceDriver, s.cfg.DatabaseURL, func(p xinghaimigrate.Progress) {
		s.migration.mu.Lock()
		s.migration.Step = p.Step
		s.migration.Current = p.Current
		s.migration.Total = p.Total
		s.migration.Detail = p.Detail
		s.migration.mu.Unlock()
	})
	s.migration.mu.Lock()
	defer s.migration.mu.Unlock()
	if s.migrationCancel != nil {
		s.migrationCancel()
		s.migrationCancel = nil
	}
	if err != nil {
		s.migration.Status = "failed"
		s.migration.Error = redactMigrationError(err.Error(), sourceDSN, s.cfg.DatabaseURL)
		s.migration.FinishedAt = time.Now()
		return
	}
	s.migration.Status = "completed"
	s.migration.FinishedAt = time.Now()
}

func (s *Service) getMigrationStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.migrationSnapshot())
}
