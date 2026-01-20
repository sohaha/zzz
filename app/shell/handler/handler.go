package handler

import (
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sohaha/zzz/app/shell/auth"
	"github.com/sohaha/zzz/app/shell/session"
	"github.com/sohaha/zzz/app/shell/web"
	"golang.org/x/time/rate"
	"gopkg.in/olahol/melody.v1"
)

type Handler struct {
	authProvider auth.Provider
	jwtManager   *auth.JWTManager
	sessionMgr   *session.Manager
	melody       *melody.Melody
	shellPath    string
	loginLimiter *rate.Limiter
	limiterMu    sync.Mutex
	ipLimiters   map[string]*rate.Limiter
}

func NewHandler(authProvider auth.Provider, jwtManager *auth.JWTManager, shellPath string) *Handler {
	h := &Handler{
		authProvider: authProvider,
		jwtManager:   jwtManager,
		sessionMgr:   session.NewManager(),
		melody:       melody.New(),
		shellPath:    shellPath,
		loginLimiter: rate.NewLimiter(rate.Every(time.Second), 10),
		ipLimiters:   make(map[string]*rate.Limiter),
	}

	h.melody.Config.WriteWait = 10 * time.Second
	h.melody.Config.PongWait = 60 * time.Second
	h.melody.Config.PingPeriod = 54 * time.Second

	h.setupMelody()
	h.startCleanup()

	return h
}

func (h *Handler) startCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			h.sessionMgr.CleanupInactive(30 * time.Minute)
			h.cleanupLimiters()
		}
	}()
}

func (h *Handler) getIPLimiter(ip string) *rate.Limiter {
	h.limiterMu.Lock()
	defer h.limiterMu.Unlock()

	limiter, exists := h.ipLimiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(time.Minute/10), 10)
		h.ipLimiters[ip] = limiter
	}
	return limiter
}

func (h *Handler) cleanupLimiters() {
	h.limiterMu.Lock()
	defer h.limiterMu.Unlock()
	h.ipLimiters = make(map[string]*rate.Limiter)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Handle /ws/* WebSocket connections
	if strings.HasPrefix(r.URL.Path, "/ws/") {
		h.handleWebSocket(w, r)
		return
	}

	switch r.URL.Path {
	case "/api/login":
		h.handleLogin(w, r)
	case "/api/logout":
		h.handleLogout(w, r)
	case "/api/sessions":
		if r.Method == http.MethodGet {
			h.handleListSessions(w, r)
		} else if r.Method == http.MethodPost {
			h.handleCreateSession(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "/api/ws":
		h.handleWebSocket(w, r)
	default:
		// Handle /api/sessions/{id} DELETE
		if strings.HasPrefix(r.URL.Path, "/api/sessions/") && r.Method == http.MethodDelete {
			h.handleDeleteSession(w, r)
			return
		}

		// Static files
		distFS, err := fs.Sub(web.StaticFS, "dist")
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
	}
}
