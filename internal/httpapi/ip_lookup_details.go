package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

func (s *Server) ipLookupDetails(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.ipLookupMember(w, r) {
		return
	}
	if s.deps.IPLookup.Live == nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_DETAILS_UNAVAILABLE"})
		return
	}
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(65 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_DETAILS_UNAVAILABLE"})
		return
	}
	options := iplookup.LiveOptions{Refresh: r.URL.Query().Get("refresh") == "true"}
	started := false
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		options.Emit = func(event iplookup.LiveEvent) error {
			data, err := json.Marshal(event)
			if err != nil {
				return err
			}
			if !started {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("X-Accel-Buffering", "no")
				w.WriteHeader(http.StatusOK)
				started = true
			}
			if _, err := w.Write(append(append([]byte("data: "), data...), '\n', '\n')); err != nil {
				return err
			}
			return controller.Flush()
		}
	}
	result, err := s.deps.IPLookup.Live.DetailsWithOptions(r.Context(), currentUser(r).ID, r.URL.Query().Get("ip"), options)
	if err != nil {
		if !started {
			s.ipLookupError(w, r, err)
		}
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, result)
	}
}
