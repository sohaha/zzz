package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
)

type SessionInfo struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"created_at"`
}

// handleListSessions 列出当前用户的会话
// 注意：当前实现为简化版，直接返回一个新的会话 ID
// 实际会话在 WebSocket 连接时创建
func (h *Handler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 token
	token := extractToken(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Unauthorized"})
		return
	}

	_, err := h.jwtManager.Verify(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Unauthorized"})
		return
	}

	// 返回空列表，前端会自动调用 createSession
	sessions := []SessionInfo{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// handleCreateSession 创建新会话
// 注意：这里只生成会话 ID，实际 PTY 在 WebSocket 连接时创建
func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 token
	token := extractToken(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Unauthorized"})
		return
	}

	claims, err := h.jwtManager.Verify(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Unauthorized"})
		return
	}

	// 生成新的会话 ID
	sessionID := zstring.Rand(16)

	zlog.Infof("[AUDIT] 会话 ID 生成 id=%s user=%s", sessionID, claims.Username)

	sessionInfo := SessionInfo{
		ID:        sessionID,
		Username:  claims.Username,
		CreatedAt: 0, // 实际创建时间在 WebSocket 连接时设置
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionInfo)
}

// handleDeleteSession 删除会话
func (h *Handler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证 token
	token := extractToken(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	_, err := h.jwtManager.Verify(token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 从 URL 提取 session ID: /api/sessions/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sessionID := parts[0]
	h.sessionMgr.Delete(sessionID)

	zlog.Infof("[AUDIT] 会话删除请求 id=%s", sessionID)

	w.WriteHeader(http.StatusNoContent)
}

// handleLogout 登出
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 这里可以添加 token 黑名单逻辑
	// 目前简单返回成功，客户端会删除本地 token

	w.WriteHeader(http.StatusNoContent)
}

// extractToken 从请求中提取 token
func extractToken(r *http.Request) string {
	// 优先从 Authorization header 读取
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	// 备用：从 query string 读取
	return r.URL.Query().Get("token")
}
