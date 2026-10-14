package httpapi

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/go-chi/chi/v5/middleware"
	"io/fs"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type contextKey string

const userContextKey contextKey = "user"

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if browserScopedRequest(r) {
			s.requireBrowserSession(next).ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			s.writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Open VanSail Carpool from Telegram to continue.")
			return
		}
		user, err := s.deps.Accounts.UserBySession(r.Context(), cookie.Value)
		if err != nil {
			s.writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Your session has expired. Open the Mini App again.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := currentUser(r)
		if user.Role != "admin" || !s.isAdminTelegramID(user.TelegramID) {
			s.writeError(w, r, http.StatusForbidden, "ADMIN_REQUIRED", "Administrator access is required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) isAdminTelegramID(telegramID int64) bool {
	_, ok := s.adminTelegramIDs[telegramID]
	return ok
}

func currentUser(r *http.Request) model.User {
	user, _ := r.Context().Value(userContextKey).(model.User)
	return user
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.deps.Logger.Error("HTTP panic", "request_id", middleware.GetReqID(r.Context()), "method", r.Method, "path", r.URL.Path, "panic", recovered, "stack", string(debug.Stack()))
				s.writeError(w, r, http.StatusInternalServerError, "INTERNAL", "The request could not be completed.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://telegram.org https://challenges.cloudflare.com; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' https://challenges.cloudflare.com; frame-src 'self' https://challenges.cloudflare.com https://www.openstreetmap.org; frame-ancestors https://web.telegram.org https://*.telegram.org")
		if (r.URL.Path == "/admin" || r.URL.Path == "/admin.html" || strings.HasPrefix(r.URL.Path, "/admin/")) && !legacyMiniAppFrame(r) {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-src 'self' https://www.openstreetmap.org; frame-ancestors 'none'")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapper := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapper, r)
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/v1/webhooks/bepusdt/") {
			path = "/api/v1/webhooks/bepusdt/[redacted]"
		}
		s.deps.Logger.Info("HTTP request", "request_id", middleware.GetReqID(r.Context()), "method", r.Method, "path", path, "status", wrapper.Status(), "duration_ms", time.Since(start).Milliseconds())
	})
}

func spaHandler(content fs.FS) http.Handler {
	files := http.FileServer(http.FS(content))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if file, err := content.Open(path); err == nil {
			if err := file.Close(); err != nil {
				http.Error(w, "failed to close file", http.StatusInternalServerError)
				return
			}
			setStaticCacheHeaders(w, path)
			files.ServeHTTP(w, r)
			return
		}
		entry := "index.html"
		if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") {
			entry = "admin.html"
		}
		index, err := fs.ReadFile(content, entry)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		setStaticCacheHeaders(w, entry)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}

func setStaticCacheHeaders(w http.ResponseWriter, path string) {
	if path == "index.html" || path == "admin.html" {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		return
	}
	if strings.HasPrefix(path, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
}
