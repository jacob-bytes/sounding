package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jacob-bytes/sounding/internal/auth"
)

// LoginEndpoint 登录/认证端点（JWT）。
type LoginEndpoint struct {
	Username string
	Password string
	Secret   string
	TTL      time.Duration
}

// NewLoginEndpoint 构建登录端点。
func NewLoginEndpoint(user, pass, secret string) *LoginEndpoint {
	if secret == "" {
		secret = "sounding-dev-secret"
	}
	return &LoginEndpoint{Username: user, Password: pass, Secret: secret, TTL: 24 * time.Hour}
}

// ServeHTTP POST /api/login  {username,password} → {token}。
func (h *LoginEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	if body.Username != h.Username || body.Password != h.Password {
		writeJSON(w, map[string]any{"status": "error", "message": "用户名或密码错误"})
		return
	}
	token, err := auth.Issue(h.Secret, body.Username, h.TTL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"status": "success", "data": map[string]any{"token": token, "expires_in": int(h.TTL.Seconds())}})
}

// AuthMiddleware 校验 Bearer JWT（可选启用）。
func (h *LoginEndpoint) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("Authorization")
		tok = strings.TrimPrefix(tok, "Bearer ")
		if tok == "" {
			tok = r.Header.Get("X-Admin-Token")
		}
		if _, err := auth.Verify(h.Secret, tok); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
