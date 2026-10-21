package httpapi

import (
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Called only after bearer identity and active subscription access are validated.
func (s *Server) subscriptionSpecialResponse(w http.ResponseWriter, r *http.Request, metadata xnode.SubscriptionMetadata) bool {
	switch metadata.ResponseKind {
	case "empty-http":
		if metadata.StatusCode < 200 || metadata.StatusCode > 599 {
			s.xnodeError(w, r, xnode.ErrInvalid)
			return true
		}
		w.Header().Del("Content-Type")
		w.Header().Del("Content-Disposition")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(metadata.StatusCode)
		return true
	case "browser":
		if strings.HasPrefix(r.URL.Path, "/api/v1/subscriptions/") {
			http.Redirect(w, r, "/subscription/"+chiURLParam(r, "token"), http.StatusSeeOther)
		} else if s.deps.Static != nil {
			spaHandler(s.deps.Static).ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
		return true
	}
	return false
}
