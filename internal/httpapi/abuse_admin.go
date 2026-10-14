package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

type abuseRuleRequest struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	QPSLimit   int    `json:"qpsLimit"`
	Enabled    bool   `json:"enabled"`
	Revision   int    `json:"revision"`
}
type abuseWhitelistRequest struct {
	Enabled bool `json:"enabled"`
}
type abusePunishmentRequest struct {
	Enabled           bool `json:"enabled"`
	IncidentThreshold int  `json:"incidentThreshold"`
	DurationMinutes   int  `json:"durationMinutes"`
	AllNodes          bool `json:"allNodes"`
	Revision          int  `json:"revision"`
}

func (s *Server) adminAbuseNodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Abuse.SyncNodes(r.Context(), s.deps.AbuseVault, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) adminCopyAbuseNodeKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.deps.Abuse.CopyNodeKey(r.Context(), s.deps.AbuseVault, chiURLParam(r, "id"))
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}
func (s *Server) adminRotateAbuseNodeKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdempotencyKey(w, r); !ok {
		return
	}
	key, err := s.deps.Abuse.RotateNodeKey(r.Context(), s.deps.AbuseVault, chiURLParam(r, "id"), time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}
func (s *Server) adminAbuseRules(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Abuse.Rules(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) adminSaveAbuseRule(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdempotencyKey(w, r); !ok {
		return
	}
	var input abuseRuleRequest
	if err := decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ABUSE_RULE", "A complete domain rule is required.")
		return
	}
	value, err := s.deps.Abuse.SaveRule(r.Context(), currentUser(r).ID, abuse.DomainRule{ID: chiURLParam(r, "id"), Name: input.Name, Expression: input.Expression, QPSLimit: input.QPSLimit, Enabled: input.Enabled, Revision: input.Revision}, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) adminDeleteAbuseRule(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdempotencyKey(w, r); !ok {
		return
	}
	revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
	if err != nil || revision < 0 {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ABUSE_RULE", "A current rule revision is required.")
		return
	}
	if err := s.deps.Abuse.DeleteRule(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), revision, time.Now().UTC()); err != nil {
		s.adminFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) adminAbuseWhitelist(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Abuse.Whitelist(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) adminSetAbuseWhitelist(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdempotencyKey(w, r); !ok {
		return
	}
	var input abuseWhitelistRequest
	if err := decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ABUSE_WHITELIST", "A whitelist value is required.")
		return
	}
	if err := s.deps.Abuse.SetWhitelist(r.Context(), chiURLParam(r, "id"), input.Enabled, time.Now().UTC()); err != nil {
		s.adminFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) adminAbusePunishments(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Abuse.Punishments(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) adminSaveAbusePunishment(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdempotencyKey(w, r); !ok {
		return
	}
	var input abusePunishmentRequest
	if err := decodeJSON(w, r, &input); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ABUSE_PUNISHMENT", "A complete punishment rule is required.")
		return
	}
	value, err := s.deps.Abuse.SavePunishment(r.Context(), currentUser(r).ID, abuse.PunishmentRule{Action: abuse.Action(chiURLParam(r, "action")), Enabled: input.Enabled, IncidentThreshold: input.IncidentThreshold, DurationMinutes: input.DurationMinutes, AllNodes: input.AllNodes, Revision: input.Revision}, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) adminAbuseStatistics(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.Abuse.Statistics(r.Context(), time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) adminAbuseRecords(w http.ResponseWriter, r *http.Request) {
	page, err := s.deps.Store.AdminAbuseRecords(r.Context(), r.URL.Query().Get("cursor"), 25)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
func (s *Server) adminDeleteAbuseRecord(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdempotencyKey(w, r); !ok {
		return
	}
	if err := s.deps.Abuse.DeleteRecord(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), time.Now().UTC()); err != nil {
		s.adminFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
