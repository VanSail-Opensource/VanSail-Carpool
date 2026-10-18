package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

func subscriptionContentFormat(r *http.Request) string {
	if format := r.URL.Query().Get("format"); format != "" {
		return format
	}
	return xnode.DetectSubscriptionFormat(r.UserAgent())
}

func subscriptionWantsHTML(r *http.Request) bool {
	if r.URL.Query().Get("format") != "" || subscriptionContentFormat(r) != "base64" {
		return false
	}
	html, other := 0.0, 0.0
	for _, entry := range strings.Split(r.Header.Get("Accept"), ",") {
		media, params, err := mime.ParseMediaType(entry)
		if err != nil {
			continue
		}
		quality := 1.0
		if text, ok := params["q"]; ok {
			quality, err = strconv.ParseFloat(text, 64)
			if err != nil || quality < 0 || quality > 1 {
				continue
			}
		}
		if media == "text/html" {
			if quality > html {
				html = quality
			}
		} else if quality > other {
			other = quality
		}
	}
	return html > 0 && html >= other
}

// The copied landing URL remains a subscription URL for native clients. Normal
// browser navigation keeps the independent public page and its format controls.
func (s *Server) subscriptionLanding(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept, User-Agent")
	privateSubscriptionHeaders(w)
	wantsHTML := subscriptionWantsHTML(r)
	if wantsHTML && s.deps.XNode != nil {
		config, err := s.deps.XNode.SubscriptionTemplates(r.Context())
		if err != nil {
			s.xnodeError(w, r, err)
			return
		}
		selection, err := xnode.SelectSubscription(config, "", r.UserAgent())
		// A configured regex selects its exact template even for HTML-capable clients.
		if selection.Source == "regex" {
			if err != nil {
				s.xnodeError(w, r, err)
				return
			}
			wantsHTML = false
		}
	}
	if wantsHTML {
		if s.deps.Static == nil {
			http.NotFound(w, r)
			return
		}
		spaHandler(s.deps.Static).ServeHTTP(w, r)
		return
	}
	s.xnodeBearerSubscription(w, r)
}
