package httpapi

import (
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountCommunity(router chi.Router) {
	router.Get("/api/v1/community/membership/check", s.communityAccess)
	router.Post("/api/v1/community/membership/check", s.communityMembershipCheck)
	router.Post("/api/v1/community/invites/{kind}", s.createCommunityInvite)
	router.Get("/api/v1/activity", s.activityOverview)
	router.Post("/api/v1/activity/check-ins", s.activityCheckIn)
	router.Post("/api/v1/activity/bets", s.activityBet)
	router.Post("/api/v1/activity/draws", s.activityDraw)
	router.Get("/api/v1/coupons/wallet", s.couponWallet)
	router.Delete("/api/v1/coupons/wallet/{id}", s.couponDiscard)
	router.Post("/api/v1/coupons/redeem", s.couponRedeem)
}

func (s *Server) mountCommunityAdmin(router chi.Router) {
	router.Get("/activity-settings", s.adminActivitySettings)
	router.Put("/activity-settings", s.adminUpdateActivitySettings)
	router.Get("/activity-games", s.adminActivityGames)
	router.Post("/activity-games", s.adminCreateActivityGame)
	router.Put("/activity-games/{id}", s.adminUpdateActivityGame)
	router.Delete("/activity-games/{id}", s.adminDeleteActivityGame)
	router.Get("/activity-games/{id}/statistics", s.adminActivityGameStatistics)
	router.Get("/lucky-draw", s.adminLuckyDraws)
	router.Post("/lucky-draw", s.adminCreateLuckyDraw)
	router.Put("/lucky-draw/{id}", s.adminUpdateLuckyDraw)
	router.Delete("/lucky-draw/{id}", s.adminDeleteLuckyDraw)
	router.Post("/lucky-draw/{id}/publish", s.adminPublishLuckyDraw)
	router.Get("/lucky-draw/{id}/statistics", s.adminLuckyDrawStatistics)
	router.Get("/lucky-draw/{id}/forecast", s.adminLuckyDrawForecast)
	router.Get("/coupons", s.adminCoupons)
	router.Post("/coupons", s.adminCreateCoupon)
	router.Put("/coupons/{id}", s.adminUpdateCoupon)
	router.Delete("/coupons/{id}", s.adminDeactivateCoupon)
}
