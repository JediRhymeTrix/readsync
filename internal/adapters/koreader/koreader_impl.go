// internal/adapters/koreader/koreader_impl.go
//
// KOReader KOSync-compatible HTTP adapter.
// Auth: username + md5(password) headers. Storage: bcrypt(md5Key).

package koreader

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"golang.org/x/time/rate"
)

// Config holds adapter configuration.
type Config struct {
	// BindAddr is the TCP listen address. Default: "127.0.0.1:7200".
	BindAddr string

	// RoutePrefix is the URL prefix for all KOSync routes.
	// Default "" means routes at /users/create, /syncs/progress (KOSync spec).
	RoutePrefix string

	// RegistrationOpen allows new user registrations.
	RegistrationOpen bool

	// TrustedProxies lists CIDR ranges for reverse-proxy header trust.
	TrustedProxies []string
}

// DefaultConfig returns a safe default config (loopback, registration closed).
func DefaultConfig() Config {
	return Config{BindAddr: "127.0.0.1:7200", RegistrationOpen: false}
}

// Adapter is the KOReader sync adapter. Implements adapters.EventEmitter.
type Adapter struct {
	cfg      Config
	db       *sql.DB
	pipeline *core.Pipeline
	log      *logging.Logger

	server    *http.Server
	mu        sync.Mutex
	health    model.AdapterHealthState
	authFails atomic.Int64

	limiterMu sync.Mutex
	limiters  map[string]*rate.Limiter
}

// New creates a KOReader adapter with the given config and database.
func New(cfg Config, db *sql.DB, log *logging.Logger) *Adapter {
	return &Adapter{
		cfg:      cfg,
		db:       db,
		log:      log,
		health:   model.HealthOK,
		limiters: make(map[string]*rate.Limiter),
	}
}

func (a *Adapter) Source() model.Source           { return model.SourceKOReader }
func (a *Adapter) SetPipeline(p *core.Pipeline)   { a.pipeline = p }

func (a *Adapter) Health() model.AdapterHealthState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.health
}

func (a *Adapter) Start(ctx context.Context) error {
	if a.pipeline == nil {
		return fmt.Errorf("koreader adapter: pipeline not set")
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	if len(a.cfg.TrustedProxies) > 0 {
		if err := router.SetTrustedProxies(a.cfg.TrustedProxies); err != nil {
			return fmt.Errorf("koreader: trusted proxies: %w", err)
		}
	} else {
		_ = router.SetTrustedProxies(nil)
	}
	a.registerRoutes(router.Group(a.cfg.RoutePrefix))
	ln, err := net.Listen("tcp", a.cfg.BindAddr)
	if err != nil {
		return fmt.Errorf("koreader: listen %s: %w", a.cfg.BindAddr, err)
	}
	a.server = &http.Server{
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if serr := a.server.Serve(ln); serr != nil && serr != http.ErrServerClosed {
			a.log.Error("koreader: serve error", logging.F("error", serr))
			a.setHealth(model.HealthFailed)
		}
	}()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutCtx)
	}()
	a.log.Info("koreader: started", logging.F("addr", a.cfg.BindAddr))
	a.setHealth(model.HealthOK)
	return nil
}

func (a *Adapter) Stop() error {
	if a.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.server.Shutdown(ctx)
}

func (a *Adapter) WriteProgress(_ context.Context, _ *model.OutboxJob) error {
	return fmt.Errorf("koreader adapter: WriteProgress not supported")
}

// RegisterTestRoutes wires the KOSync routes onto an externally-provided
// router group.  Used by tests that want to exercise handlers with
// httptest.NewRecorder without starting a real TCP listener.
func (a *Adapter) RegisterTestRoutes(g *gin.RouterGroup) {
	a.registerRoutes(g)
}

func (a *Adapter) registerRoutes(g *gin.RouterGroup) {
	g.POST("/users/create", a.rateLimitMiddleware, a.handleRegister)
	g.GET("/users/auth", a.rateLimitMiddleware, a.authMiddleware, a.handleAuth)
	g.PUT("/syncs/progress", a.authMiddleware, a.handlePush)
	g.GET("/syncs/progress/:document", a.authMiddleware, a.handlePull)
}

func (a *Adapter) setHealth(s model.AdapterHealthState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.health = s
}

func (a *Adapter) recordAuthFailure() {
	n := a.authFails.Add(1)
	if n >= 10 {
		a.setHealth(model.HealthNeedsUserAction)
		a.log.Warn("koreader: repeated auth failures", logging.F("count", n))
	}
}

func (a *Adapter) clearAuthFailures() {
	a.authFails.Store(0)
	a.setHealth(model.HealthOK)
}

func (a *Adapter) rateLimitMiddleware(c *gin.Context) {
	if !a.getLimiter(c.ClientIP()).Allow() {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "Too many requests."})
		return
	}
	c.Next()
}

func (a *Adapter) getLimiter(ip string) *rate.Limiter {
	a.limiterMu.Lock()
	defer a.limiterMu.Unlock()
	if l, ok := a.limiters[ip]; ok {
		return l
	}
	l := rate.NewLimiter(rate.Every(6*time.Second), 5)
	a.limiters[ip] = l
	return l
}
