package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zzz/app/shell/terminal"
	"gopkg.in/olahol/melody.v1"
)

type Session struct {
	ID         string
	Username   string
	PTY        *terminal.PTY
	WS         *melody.Session
	CreatedAt  time.Time
	LastActive time.Time
	MouseMode  bool
	mu         sync.Mutex
	closeOnce  sync.Once
}

type Manager struct {
	sessions sync.Map
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Create(id, username string, pty *terminal.PTY, ws *melody.Session) *Session {
	session := &Session{
		ID:         id,
		Username:   username,
		PTY:        pty,
		WS:         ws,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		MouseMode:  false,
	}

	m.sessions.Store(id, session)
	zlog.Infof("[AUDIT] 会话创建 id=%s user=%s", id, username)
	return session
}

func (m *Manager) Get(id string) (*Session, error) {
	value, ok := m.sessions.Load(id)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	session, ok := value.(*Session)
	if !ok {
		return nil, fmt.Errorf("invalid session type")
	}

	return session, nil
}

func (m *Manager) Delete(id string) {
	if value, ok := m.sessions.LoadAndDelete(id); ok {
		if session, ok := value.(*Session); ok {
			session.closeOnce.Do(func() {
				if session.PTY != nil {
					session.PTY.Close()
				}
				zlog.Infof("[AUDIT] 会话关闭 id=%s user=%s", session.ID, session.Username)
			})
		}
	}
}

func (m *Manager) UpdateActivity(id string) error {
	value, ok := m.sessions.Load(id)
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session, ok := value.(*Session); ok {
		session.mu.Lock()
		session.LastActive = time.Now()
		session.mu.Unlock()
	}
	return nil
}

func (m *Manager) SetMouseMode(id string, enabled bool) error {
	value, ok := m.sessions.Load(id)
	if !ok {
		return fmt.Errorf("session not found")
	}
	if session, ok := value.(*Session); ok {
		session.mu.Lock()
		session.MouseMode = enabled
		session.mu.Unlock()
	}
	return nil
}

func (m *Manager) Count() int {
	count := 0
	m.sessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

func (m *Manager) CleanupInactive(timeout time.Duration) {
	now := time.Now()
	cleaned := 0
	m.sessions.Range(func(key, value interface{}) bool {
		if session, ok := value.(*Session); ok {
			session.mu.Lock()
			lastActive := session.LastActive
			session.mu.Unlock()

			if now.Sub(lastActive) > timeout {
				m.Delete(session.ID)
				cleaned++
			}
		}
		return true
	})
	if cleaned > 0 {
		zlog.Infof("[AUDIT] 清理不活跃会话 count=%d", cleaned)
	}
}
