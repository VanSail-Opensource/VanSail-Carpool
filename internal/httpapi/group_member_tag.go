package httpapi

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"net/http"
)

func (s *Server) groupMemberTag(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	value, err := s.deps.Accounts.GroupMemberTag(r.Context(), user)
	if err != nil {
		s.groupMemberTagError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateGroupMemberTag(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	var input struct {
		Tag *string `json:"tag"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.Tag == nil {
		s.writeError(w, r, 400, "INVALID_GROUP_TAG", "Provide a valid member tag.")
		return
	}
	value, err := s.deps.Accounts.SetGroupMemberTag(r.Context(), user, *input.Tag)
	if err != nil {
		s.groupMemberTagError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) groupMemberTagError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, accounts.ErrInvalidGroupTag) {
		s.writeError(w, r, 422, "INVALID_GROUP_TAG", "Use at most 16 characters without emoji.")
		return
	}
	if errors.Is(err, accounts.ErrGroupTagUnavailable) {
		s.writeError(w, r, 409, "GROUP_TAG_UNAVAILABLE", "Group membership or bot permissions prevent this change.")
		return
	}
	s.deps.Logger.Warn("Telegram member tag unavailable", "error", err)
	s.writeError(w, r, 502, "GROUP_TAG_UNAVAILABLE", "Telegram could not apply the tag. Use at most 16 characters without emoji.")
}
