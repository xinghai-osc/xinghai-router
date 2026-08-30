package app

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Service struct {
	cfg                   Config
	db                    *pgxpool.Pool
	httpClient            *http.Client
	streamClient          *http.Client
	limiter               rateLimiter
	ipLimiter             rateLimiter
	rankingsLimiter       rateLimiter
	performanceLimiter    rateLimiter
	background            *backgroundWriter
	pricingCache          *ttlCache[string, pricingRule]
	groupCache            *ttlCache[string, float64]
	groupConcurrencyCache *ttlCache[string, int]
	userConcurrencyCache  *ttlCache[string, int]
	groupLimiter          *GroupLimiter
	userLimiter           *GroupLimiter
	reliabilityData       *ttlCache[struct{}, reliabilitySettings]
	contentPolicyData     *ttlCache[struct{}, contentPolicySnapshot]
	conversationCacheData *ttlCache[struct{}, conversationCacheSettings]
	rankingsCache         *ttlCache[string, rankingsPayload]
	performanceCache      *ttlCache[string, modelPerformancePayload]
	channelCache          *ttlCache[channelRouteKey, []channel]
	channelKeyCache       *ttlCache[int64, []channelKeyCredential]
	subscriptionCache     *ttlCache[subscriptionRouteKey, subscriptionAccess]
	channelQuotaCache     *ttlCache[int64, bool]
	quotaAbsentCache      *ttlCache[quotaRouteKey, struct{}]
	promptCache           *promptPrefixCache
	keyTouchCache         *ttlCache[string, struct{}]
	scheduler             context.CancelFunc
	migration             migrationStatus
	migrationCancel       context.CancelFunc
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// newPool sizes the connection pool for gateway traffic: the stdlib default of
// max(4, NumCPU) conns serialises the several statements each proxied request issues.
func newPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	// An explicit pool_max_conns in DATABASE_URL always wins; DB_MAX_CONNS is the
	// env override; otherwise scale with the CPU count.
	if maxConns := cfg.DBMaxConns; maxConns > 0 {
		poolCfg.MaxConns = int32(maxConns)
	} else if !strings.Contains(cfg.DatabaseURL, "pool_max_conns") {
		maxConns = 8 * runtime.GOMAXPROCS(0)
		if maxConns < 16 {
			maxConns = 16
		}
		if maxConns > 64 {
			maxConns = 64
		}
		poolCfg.MaxConns = int32(maxConns)
	}
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnLifetimeJitter = 5 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute
	return pgxpool.NewWithConfig(ctx, poolCfg)
}

func New(ctx context.Context, cfg Config) (*Service, error) {
	db, err := newPool(ctx, cfg)
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
	if err := setTrustedProxies(cfg.TrustedProxies); err != nil {
		db.Close()
		return nil, fmt.Errorf("trusted proxies: %w", err)
	}
	if cfg.TrustedProxies != "" {
		log.Printf("trusted proxies enabled: %s", cfg.TrustedProxies)
	}
	limiter, mode := newRateLimiter(cfg.RedisURL, cfg.RateLimitPerMinute)
	ipLimiter, ipMode := newRateLimiter(cfg.RedisURL, cfg.IPRateLimitPerMinute)
	rankingsLimiter, _ := newRateLimiter(cfg.RedisURL, rankingsPerMinute)
	performanceLimiter, _ := newRateLimiter(cfg.RedisURL, performancePerMinute)
	if mode == "redis" || ipMode == "redis" {
		log.Printf("rate limiter backend: redis (memory fallback on redis errors)")
	} else {
		log.Printf("rate limiter backend: memory")
	}
	s := &Service{
		cfg:                   cfg,
		db:                    db,
		httpClient:            newHTTPClient(cfg.RequestTimeout),
		streamClient:          newStreamClient(0),
		limiter:               limiter,
		ipLimiter:             ipLimiter,
		rankingsLimiter:       rankingsLimiter,
		performanceLimiter:    performanceLimiter,
		background:            newBackgroundWriter(),
		pricingCache:          newTTLCache[string, pricingRule](pricingCacheTTL),
		groupCache:            newTTLCache[string, float64](groupCacheTTL),
		groupConcurrencyCache: newTTLCache[string, int](groupCacheTTL),
		userConcurrencyCache:  newTTLCache[string, int](groupCacheTTL),
		groupLimiter:          NewGroupLimiter(),
		userLimiter:           NewGroupLimiter(),
		reliabilityData:       newTTLCache[struct{}, reliabilitySettings](reliabilityCacheTTL),
		contentPolicyData:     newTTLCache[struct{}, contentPolicySnapshot](reliabilityCacheTTL),
		conversationCacheData: newTTLCache[struct{}, conversationCacheSettings](reliabilityCacheTTL),
		rankingsCache:         newTTLCache[string, rankingsPayload](rankingsCacheTTL),
		performanceCache:      newTTLCache[string, modelPerformancePayload](performanceCacheTTL),
		channelCache:          newTTLCache[channelRouteKey, []channel](channelCacheTTL),
		channelKeyCache:       newTTLCache[int64, []channelKeyCredential](channelCacheTTL),
		subscriptionCache:     newTTLCache[subscriptionRouteKey, subscriptionAccess](subscriptionCacheTTL),
		channelQuotaCache:     newTTLCache[int64, bool](quotaCacheTTL),
		quotaAbsentCache:      newTTLCache[quotaRouteKey, struct{}](quotaCacheTTL),
		promptCache:           newPromptPrefixCache(cfg.LocalPromptCache, cfg.LocalPromptCacheSize),
		keyTouchCache:         newTTLCache[string, struct{}](keyTouchInterval),
		migration:             migrationStatus{mu: &sync.Mutex{}},
	}
	if err := s.bootstrapAdmin(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("bootstrap admin: %w", err)
	}
	schedulerCtx, cancel := context.WithCancel(context.Background())
	s.scheduler = cancel
	s.startHealthCheckScheduler(schedulerCtx)
	s.startChannelBalanceScheduler(schedulerCtx)
	s.startWalletSettlementScheduler(schedulerCtx)
	s.startAuthCleanupScheduler(schedulerCtx)
	if err := os.MkdirAll(s.cfg.ConversationCacheDir, 0o755); err != nil {
		log.Printf("conversation cache dir: %v", err)
	}
	s.startConversationCacheCleanup(schedulerCtx)
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
			s.rankingsLimiter.cleanup()
			s.performanceLimiter.cleanup()
		}
	}
}
func (s *Service) Close() {
	if s.scheduler != nil {
		s.scheduler()
	}
	s.background.close()
	if s.limiter != nil {
		s.limiter.close()
	}
	if s.rankingsLimiter != nil {
		s.rankingsLimiter.close()
	}
	if s.performanceLimiter != nil {
		s.performanceLimiter.close()
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
