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
	ID         string    `json:"id"`
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
	var id string
	err := s.db.QueryRow(context.Background(),
		`insert into migration_requests(status, source_driver, step, started_at) values('running', $1, 'connect', now()) returning id`,
		sourceDriver).Scan(&id)
	if err != nil {
		return false
	}
	s.migration = migrationStatus{mu: &sync.Mutex{}, ID: id, Status: "running", Step: "connect", Current: 0, Total: 0, Detail: "Connecting to source and target databases", StartedAt: time.Now()}
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
	err := xinghaimigrate.Run(ctx, sourceDSN, sourceDriver, s.cfg.DatabaseURL, s.cfg.EncryptionKey, func(p xinghaimigrate.Progress) {
		s.migration.mu.Lock()
		s.migration.Step = p.Step
		s.migration.Current = p.Current
		s.migration.Total = p.Total
		s.migration.Detail = p.Detail
		s.migration.mu.Unlock()
	})
	s.migration.mu.Lock()
	if s.migrationCancel != nil {
		s.migrationCancel()
		s.migrationCancel = nil
	}
	var status, errMsg string
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		s.migration.Status = "failed"
		s.migration.Error = errMsg
		s.migration.FinishedAt = time.Now()
	} else {
		status = "completed"
		s.migration.Status = "completed"
		s.migration.FinishedAt = time.Now()
	}
	id := s.migration.ID
	step := s.migration.Step
	current := s.migration.Current
	total := s.migration.Total
	detail := s.migration.Detail
	finishedAt := s.migration.FinishedAt
	s.migration.mu.Unlock()

	if id != "" {
		s.db.Exec(context.Background(),
			`update migration_requests set status=$1, step=$2, current=$3, total=$4, detail=$5, error=$6, finished_at=$7 where id=$8`,
			status, step, current, total, detail, errMsg, finishedAt, id)
	}
}

func (s *Service) getMigrationStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.migrationSnapshot())
}

func (s *Service) listMigrationRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `select id,status,source_driver,step,current,total,detail,error,started_at,finished_at,created_at from migration_requests order by created_at desc limit 50`)
	if err != nil {
		writeError(w, 500, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := []map[string]any{}
	for rows.Next() {
		var id, status, driver, step, detail, errMsg string
		var current, total int
		var startedAt, createdAt any
		var finishedAt any
		if rows.Scan(&id, &status, &driver, &step, &current, &total, &detail, &errMsg, &startedAt, &finishedAt, &createdAt) == nil {
			data = append(data, map[string]any{
				"id": id, "status": status, "source_driver": driver,
				"step": step, "current": current, "total": total,
				"detail": detail, "error": errMsg,
				"started_at": startedAt, "finished_at": finishedAt, "created_at": createdAt,
			})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
