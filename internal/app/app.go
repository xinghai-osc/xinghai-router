package app

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Service struct {
	cfg             Config
	db              *pgxpool.Pool
	httpClient      *http.Client
	limiter         *limiter
	ipLimiter       *limiter
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
	s := &Service{cfg: cfg, db: db, httpClient: &http.Client{Timeout: cfg.RequestTimeout}, limiter: newLimiter(cfg.RateLimitPerMinute), ipLimiter: newLimiter(cfg.IPRateLimitPerMinute)}
	schedulerCtx, cancel := context.WithCancel(context.Background())
	s.scheduler = cancel
	s.startHealthCheckScheduler(schedulerCtx)
	go s.limiterCleanup(schedulerCtx)
	return s, nil
}
func (s *Service) limiterCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.limiter.cleanup()
			s.ipLimiter.cleanup()
		}
	}
}
func (s *Service) Close() {
	if s.scheduler != nil {
		s.scheduler()
	}
	s.db.Close()
}
func (s *Service) Handler() http.Handler { return s.routes() }
