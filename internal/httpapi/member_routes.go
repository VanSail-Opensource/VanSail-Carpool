package httpapi

import "github.com/go-chi/chi/v5"

func (s *Server) mountMemberOperations(router chi.Router) {
	router.Get("/api/v1/subscription", s.memberSubscription)
	router.Get("/api/v1/me/preferences", s.userPreferences)
	router.Patch("/api/v1/me/preferences", s.updateUserPreferences)
	router.Get("/api/v1/me/group-member-tag", s.groupMemberTag)
	router.Put("/api/v1/me/group-member-tag", s.updateGroupMemberTag)
	router.Get("/api/v1/me/traffic-reset-automation", s.trafficResetAutomation)
	router.Put("/api/v1/me/traffic-reset-automation", s.updateTrafficResetAutomation)
	router.Get("/api/v1/me/internal-squads", s.memberComboControls)
	router.Put("/api/v1/me/internal-squads/{uuid}", s.memberSwitchSquad)
	router.Get("/api/v1/purchases/{id}/early-activation", s.memberEarlyActivationQuote)
	router.Post("/api/v1/purchases/{id}/activate", s.memberActivateEarly)
	router.Get("/api/v1/purchases/{id}/refund", s.memberRefundQuote)
	router.Post("/api/v1/purchases/{id}/refund", s.memberRefund)
	router.Post("/api/v1/subscription/connections", s.requestConnections)
	router.Get("/api/v1/subscription/connections/{id}", s.pollConnections)
	router.Post("/api/v1/subscription/connections/drop", s.dropConnection)
	router.Get("/api/v1/subscription/ip-blocks", s.memberIPBlocks)
	router.Post("/api/v1/subscription/ip-blocks/{blockId}/unblock", s.memberUnblockIP)
	router.Get("/api/v1/operations/{id}", s.memberOperation)
}
