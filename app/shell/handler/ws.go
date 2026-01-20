package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zzz/app/shell/session"
	"github.com/sohaha/zzz/app/shell/terminal"
	"gopkg.in/olahol/melody.v1"
)

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type ResizeData struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type MouseModeData struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) setupMelody() {
	h.melody.HandleConnect(func(s *melody.Session) {
		sessionID, ok := s.Get("sessionID")
		if !ok {
			return
		}
		username, ok := s.Get("username")
		if !ok {
			return
		}
		ptyObj, ok := s.Get("pty")
		if !ok {
			return
		}
		pty, ok := ptyObj.(*terminal.PTY)
		if !ok {
			return
		}

		sess := h.sessionMgr.Create(sessionID.(string), username.(string), pty, s)
		go h.readPTYOutput(sess)
	})

	h.melody.HandleDisconnect(func(s *melody.Session) {
		if sessionID, ok := s.Get("sessionID"); ok {
			h.sessionMgr.Delete(sessionID.(string))
		}
	})

	h.melody.HandleMessage(func(s *melody.Session, msg []byte) {
		var wsMsg WSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			zlog.Warnf("解析 WebSocket 消息失败: %v", err)
			return
		}

		sessionID, ok := s.Get("sessionID")
		if !ok {
			return
		}

		sess, err := h.sessionMgr.Get(sessionID.(string))
		if err != nil {
			return
		}

		switch wsMsg.Type {
		case "input":
			var input string
			if err := json.Unmarshal(wsMsg.Data, &input); err != nil {
				zlog.Warnf("解析输入数据失败: %v", err)
				return
			}
			sess.PTY.Write([]byte(input))

		case "resize":
			var data ResizeData
			if err := json.Unmarshal(wsMsg.Data, &data); err != nil {
				zlog.Warnf("解析 resize 数据失败: %v", err)
				return
			}
			sess.PTY.Resize(data.Rows, data.Cols)

		case "mouse":
			var data MouseModeData
			if err := json.Unmarshal(wsMsg.Data, &data); err != nil {
				zlog.Warnf("解析 mouse 数据失败: %v", err)
				return
			}
			h.sessionMgr.SetMouseMode(sessionID.(string), data.Enabled)
		}

		h.sessionMgr.UpdateActivity(sessionID.(string))
	})
}

func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 优先从 query string 获取 token（前端实现方式）
	token := r.URL.Query().Get("token")

	// 如果 query string 没有，尝试从 Authorization header 获取
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}

	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtManager.Verify(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pty, err := terminal.NewPTY(h.shellPath, 24, 80)
	if err != nil {
		http.Error(w, "Failed to create terminal", http.StatusInternalServerError)
		return
	}

	sessionID := extractSessionIDFromPath(r.URL.Path)
	if sessionID == "" {
		sessionID = zstring.Rand(16)
	}

	err = h.melody.HandleRequestWithKeys(w, r, map[string]interface{}{
		"sessionID": sessionID,
		"username":  claims.Username,
		"pty":       pty,
	})
	if err != nil {
		pty.Close()
		return
	}
}

func extractSessionIDFromPath(path string) string {
	if !strings.HasPrefix(path, "/ws/") {
		return ""
	}
	trimmed := strings.TrimPrefix(path, "/ws/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

func (h *Handler) readPTYOutput(sess *session.Session) {
	defer sess.PTY.Close()
	defer h.sessionMgr.Delete(sess.ID)

	buf := make([]byte, 8192)
	for {
		n, err := sess.PTY.Read(buf)
		if err != nil {
			if err != io.EOF {
				zlog.Errorf("PTY 读取失败: %v", err)
			}
			break
		}

		output := string(buf[:n])
		outputJSON, err := json.Marshal(output)
		if err != nil {
			zlog.Warnf("输出序列化失败: %v", err)
			continue
		}
		msg, err := json.Marshal(WSMessage{
			Type: "output",
			Data: json.RawMessage(outputJSON),
		})
		if err != nil {
			zlog.Warnf("消息序列化失败: %v", err)
			continue
		}

		if err := sess.WS.Write(msg); err != nil {
			break
		}
	}
}
