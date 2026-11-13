package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
)

type communityMembershipResponse struct {
	ActiveCombo   bool         `json:"activeCombo"`
	GroupJoined   bool         `json:"groupJoined"`
	ChannelJoined bool         `json:"channelJoined"`
	User          userResponse `json:"user"`
}

type communityAccessResponse struct {
	ActiveCombo bool `json:"activeCombo"`
}

func (s *Server) communityAccess(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	active, err := s.deps.Accounts.CheckCommunityAccess(r.Context(), user)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "COMMUNITY_ACCESS_UNAVAILABLE", "Community access could not be checked.")
		return
	}
	writeJSON(w, http.StatusOK, communityAccessResponse{ActiveCombo: active})
}

func (s *Server) communityMembershipCheck(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	membership, err := s.deps.Accounts.CheckCommunityMembership(r.Context(), user)
	if err != nil {
		s.writeError(w, r, http.StatusBadGateway, "MEMBERSHIP_CHECK_FAILED", "Telegram membership could not be checked.")
		return
	}
	writeJSON(w, http.StatusOK, communityMembershipResponse{ActiveCombo: membership.ActiveCombo, GroupJoined: membership.GroupJoined,
		ChannelJoined: membership.ChannelJoined, User: mapUser(membership.User)})
}

func (s *Server) createCommunityInvite(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	space := accounts.CommunitySpace(chiURLParam(r, "kind"))
	if space != accounts.CommunityGroup && space != accounts.CommunityChannel {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COMMUNITY_SPACE", "Choose a valid community space.")
		return
	}
	url, expiresAt, err := s.deps.Accounts.CreateCommunityInvite(r.Context(), user, space)
	if err != nil {
		switch {
		case errors.Is(err, accounts.ErrActiveComboRequired):
			s.writeError(w, r, http.StatusConflict, "ACTIVE_COMBO_REQUIRED", "An active combo is required to join this community.")
		case errors.Is(err, accounts.ErrCommunityAlreadyJoined):
			s.writeError(w, r, http.StatusConflict, "COMMUNITY_ALREADY_JOINED", "You have already joined this community.")
		default:
			s.writeError(w, r, http.StatusServiceUnavailable, "INVITES_UNAVAILABLE", "Join links are temporarily unavailable.")
		}
		return
	}
	writeJSON(w, http.StatusOK, struct {
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expiresAt"`
	}{URL: url, ExpiresAt: expiresAt})
}
