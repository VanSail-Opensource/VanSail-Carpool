package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

type luckyDrawAdminResponse struct {
	ID                    string                        `json:"id"`
	Name                  string                        `json:"name"`
	Description           string                        `json:"description"`
	Kind                  string                        `json:"kind"`
	Status                string                        `json:"status"`
	Enabled               bool                          `json:"enabled"`
	FeeTXBMinor           string                        `json:"feeTxbMinor"`
	ExpectedParticipation int                           `json:"expectedParticipation,omitempty"`
	Threshold             int                           `json:"threshold,omitempty"`
	Keyword               string                        `json:"keyword,omitempty"`
	Command               string                        `json:"command,omitempty"`
	Seats                 int                           `json:"seats"`
	Prizes                []luckyDrawPrizeAdminResponse `json:"prizes"`
	CreatedAt             time.Time                     `json:"createdAt"`
	UpdatedAt             time.Time                     `json:"updatedAt"`
}
type luckyDrawPrizeAdminResponse struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	ProbabilityBPS int             `json:"probabilityBps,omitempty"`
	Stock          int64           `json:"stock,omitempty"`
	Reward         activity.Reward `json:"reward"`
}

func mapLuckyDrawAdmin(draw activity.LuckyDraw) luckyDrawAdminResponse {
	prizes := make([]luckyDrawPrizeAdminResponse, 0, len(draw.Prizes))
	for _, prize := range draw.Prizes {
		prizes = append(prizes, luckyDrawPrizeAdminResponse{ID: prize.ID, Name: prize.Name, ProbabilityBPS: prize.ProbabilityBPS,
			Stock: prize.Stock, Reward: prize.Reward})
	}
	return luckyDrawAdminResponse{ID: draw.ID, Name: draw.Name, Description: draw.Description, Kind: draw.Kind, Status: draw.Status,
		Enabled: draw.Enabled, FeeTXBMinor: strconv.FormatInt(draw.FeeMinor, 10), ExpectedParticipation: draw.ExpectedParticipation,
		Threshold: draw.Threshold, Keyword: draw.Keyword, Command: draw.Command, Seats: draw.Seats, Prizes: prizes, CreatedAt: draw.CreatedAt, UpdatedAt: draw.UpdatedAt}
}
func (s *Server) adminLuckyDraws(w http.ResponseWriter, r *http.Request) {
	draws, err := s.deps.Activity.Draws(r.Context(), false)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	items := make([]luckyDrawAdminResponse, 0, len(draws))
	for _, draw := range draws {
		items = append(items, mapLuckyDrawAdmin(draw))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type luckyDrawRequest struct {
	Name                  string `json:"name"`
	Description           string `json:"description"`
	Kind                  string `json:"kind"`
	Enabled               bool   `json:"enabled"`
	FeeTXBMinor           string `json:"feeTxbMinor"`
	ExpectedParticipation int    `json:"expectedParticipation"`
	Threshold             int    `json:"threshold"`
	Keyword               string `json:"keyword"`
	Command               string `json:"command"`
	Prizes                []struct {
		ID             string          `json:"id"`
		Name           string          `json:"name"`
		ProbabilityBPS int             `json:"probabilityBps"`
		Stock          int64           `json:"stock"`
		Reward         activity.Reward `json:"reward"`
	} `json:"prizes"`
}

func (s *Server) adminCreateLuckyDraw(w http.ResponseWriter, r *http.Request) {
	s.adminSaveLuckyDraw(w, r, "")
}
func (s *Server) adminUpdateLuckyDraw(w http.ResponseWriter, r *http.Request) {
	s.adminSaveLuckyDraw(w, r, chiURLParam(r, "id"))
}
func (s *Server) adminDeleteLuckyDraw(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Store.DeleteLuckyDraw(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), time.Now().UTC()); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) adminPublishLuckyDraw(w http.ResponseWriter, r *http.Request) {
	groupID, ok := s.telegramGroupID(r.Context())
	if !ok {
		s.writeError(w, r, http.StatusUnprocessableEntity, "GROUP_NOT_CONFIGURED", "Community group is not configured.")
		return
	}
	if err := s.deps.Activity.PublishRaffle(r.Context(), chiURLParam(r, "id"), groupID); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
func (s *Server) adminLuckyDrawStatistics(w http.ResponseWriter, r *http.Request) {
	from, to, bucket, location, ok := s.statisticsWindow(w, r)
	if !ok {
		return
	}
	statistics, err := s.deps.Store.LuckyDrawStatistics(r.Context(), chiURLParam(r, "id"), from, to, bucket, location)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, statistics)
}
func (s *Server) adminLuckyDrawForecast(w http.ResponseWriter, r *http.Request) {
	forecast, err := s.deps.Store.LuckyDrawForecast(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, forecast)
}
func (s *Server) adminSaveLuckyDraw(w http.ResponseWriter, r *http.Request, id string) {
	var request luckyDrawRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_LUCKY_DRAW", "Lucky-draw fields are invalid.")
		return
	}
	fee, err := parseMinorString(request.FeeTXBMinor, false)
	if err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_DRAW_FEE", "Draw fee must be positive.")
		return
	}
	prizes := make([]activity.PrizeInput, 0, len(request.Prizes))
	for _, prize := range request.Prizes {
		prizes = append(prizes, activity.PrizeInput{ID: prize.ID, Name: prize.Name, ProbabilityBPS: prize.ProbabilityBPS,
			Stock: prize.Stock, Reward: prize.Reward})
	}
	if !s.validateDrawCatalog(w, r, prizes) {
		return
	}
	draw, err := s.deps.Activity.SaveDraw(r.Context(), activity.LuckyDrawInput{ID: id, Name: request.Name, Description: request.Description,
		Kind: request.Kind, Enabled: request.Enabled, FeeMinor: fee, ExpectedParticipation: request.ExpectedParticipation,
		Threshold: request.Threshold, Keyword: request.Keyword, Command: request.Command, Prizes: prizes})
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, mapLuckyDrawAdmin(draw))
}

func (s *Server) validateDrawCatalog(w http.ResponseWriter, r *http.Request, prizes []activity.PrizeInput) bool {
	needsCombos, needsSquads := false, false
	for _, prize := range prizes {
		if prize.Reward.Kind == activity.RewardEntitlementGrant || prize.Reward.Kind == activity.RewardCoreComboSwitch {
			needsCombos = true
		}
		if len(prize.Reward.SquadUUIDs) > 0 {
			needsSquads = true
		}
	}
	if needsCombos {
		combos, err := s.deps.Store.ListCombos(r.Context(), false)
		if err != nil {
			s.communityFailure(w, r, err)
			return false
		}
		known := make(map[string]bool, len(combos))
		for _, combo := range combos {
			known[combo.ID] = combo.Active
		}
		for _, prize := range prizes {
			if prize.Reward.Kind == activity.RewardEntitlementGrant || prize.Reward.Kind == activity.RewardCoreComboSwitch {
				if !known[prize.Reward.ComboID] {
					s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_DRAW_COMBO", "Draw combo is unavailable.")
					return false
				}
			}
		}
	}
	if needsSquads {
		squads, err := s.deps.Admin.Squads(r.Context())
		if err != nil {
			s.adminFailure(w, r, err)
			return false
		}
		known := make(map[string]bool, len(squads))
		for _, squad := range squads {
			known[squad.GroupID] = true
		}
		for _, prize := range prizes {
			for _, uuid := range prize.Reward.SquadUUIDs {
				if !known[uuid] {
					s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_DRAW_SQUAD", "Draw squad is unavailable.")
					return false
				}
			}
		}
	}
	return true
}
