package httpapi

import (
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

type abusePolicyRequest struct {
	OutboundTags           []string `json:"outboundTags"`
	GlobalEnabled          bool     `json:"globalEnabled"`
	GlobalLimit            int      `json:"globalLimit"`
	StreakSeconds          *int     `json:"streakSeconds"`
	WarningValidityDays    int      `json:"warningValidityDays"`
	WarningCooldownMinutes int      `json:"warningCooldownMinutes"`
	Revision               int      `json:"revision"`
}

func (s *Server) adminAbusePolicy(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.Abuse.Policy(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) adminUpdateAbusePolicy(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdempotencyKey(w, r); !ok {
		return
	}
	var input abusePolicyRequest
	if err := decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ABUSE_POLICY", "A complete abuse policy is required.")
		return
	}
	streakSeconds, err := s.compatibleStreakSeconds(r, input.StreakSeconds)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	value, err := s.deps.Abuse.UpdatePolicy(r.Context(), currentUser(r).ID, abuse.Policy{
		OutboundTags:  input.OutboundTags,
		GlobalEnabled: input.GlobalEnabled, GlobalLimit: input.GlobalLimit,
		StreakSeconds: streakSeconds, WarningValidityDays: input.WarningValidityDays,
		WarningCooldownMinutes: input.WarningCooldownMinutes, Revision: input.Revision,
	}, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) compatibleStreakSeconds(r *http.Request, value *int) (int, error) {
	if value != nil {
		return *value, nil
	}
	current, err := s.deps.Abuse.Policy(r.Context())
	return current.StreakSeconds, err
}
