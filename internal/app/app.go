package app

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Service struct {
	cfg             Config
	db              *pgxpool.Pool
	httpClient      *http.Client
	limiter         rateLimiter
	scheduler       context.CancelFunc
	migration       migrationStatus
	migrationCancel context.CancelFunc
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureBootstrapAdmin(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := setTrustedProxies(cfg.TrustedProxies); err != nil {
		db.Close()
		return nil, fmt.Errorf("trusted proxies: %w", err)
	}
	if cfg.TrustedProxies != "" {
		log.Printf("trusted proxies enabled: %s", cfg.TrustedProxies)
	}
	limiter, mode := newRateLimiter(cfg.RedisURL, cfg.RateLimitPerMinute)
	if mode == "redis" {
		log.Printf("rate limiter backend: redis (memory fallback on redis errors)")
	} else {
		log.Printf("rate limiter backend: memory")
	}
	s := &Service{cfg: cfg, db: db, httpClient: newHTTPClient(cfg.RequestTimeout), limiter: limiter}
	schedulerCtx, cancel := context.WithCancel(context.Background())
	s.scheduler = cancel
	s.startHealthCheckScheduler(schedulerCtx)
	s.startAuthCleanupScheduler(schedulerCtx)
	return s, nil
}
func (s *Service) Close() {
	if s.scheduler != nil {
		s.scheduler()
	}
	if s.limiter != nil {
		s.limiter.close()
	}
	s.db.Close()
}
func (s *Service) Handler() http.Handler { return s.routes() }

func (s *Service) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) readyz(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	if err := s.db.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "database is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
