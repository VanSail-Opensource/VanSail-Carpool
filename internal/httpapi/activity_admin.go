package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
)

func (s *Server) adminActivitySettings(w http.ResponseWriter, r *http.Request) {
	config, err := s.activityConfig(r.Context())
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone":                   config.Timezone,
		"dailyRewardMinTxb":          txbMajorString(config.RewardMinMinor),
		"dailyRewardMinTxbMinor":     strconv.FormatInt(config.RewardMinMinor, 10),
		"dailyRewardMaxTxb":          txbMajorString(config.RewardMaxMinor),
		"dailyRewardMaxTxbMinor":     strconv.FormatInt(config.RewardMaxMinor, 10),
		"groupMessageThreshold":      config.GroupMessageThreshold,
		"groupMessageRewardTxb":      txbMajorString(config.GroupMessageRewardMinor),
		"groupMessageRewardTxbMinor": strconv.FormatInt(config.GroupMessageRewardMinor, 10),
	})
}

func (s *Server) adminUpdateActivitySettings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Timezone              string `json:"timezone"`
		DailyRewardMinTXB     string `json:"dailyRewardMinTxb"`
		DailyRewardMaxTXB     string `json:"dailyRewardMaxTxb"`
		GroupMessageThreshold int    `json:"groupMessageThreshold"`
		GroupMessageRewardTXB string `json:"groupMessageRewardTxb"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ACTIVITY_SETTINGS", "All activity settings are required.")
		return
	}
	request.Timezone = strings.TrimSpace(request.Timezone)
	request.DailyRewardMinTXB = strings.TrimSpace(request.DailyRewardMinTXB)
	request.DailyRewardMaxTXB = strings.TrimSpace(request.DailyRewardMaxTXB)
	request.GroupMessageRewardTXB = strings.TrimSpace(request.GroupMessageRewardTXB)
	if _, err := time.LoadLocation(request.Timezone); err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_TIMEZONE", "Use a valid IANA timezone such as Asia/Shanghai.")
		return
	}
	if request.GroupMessageThreshold < 0 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_GROUP_MESSAGE_THRESHOLD", "Message threshold must be a non-negative integer.")
		return
	}
	dailyRewardMinMinor, err := billing.ParseTXBMajor(request.DailyRewardMinTXB)
	if err != nil || dailyRewardMinMinor < 0 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_DAILY_REWARD_MINIMUM", "Daily reward minimum must be a non-negative TXB amount.")
		return
	}
	dailyRewardMaxMinor, err := billing.ParseTXBMajor(request.DailyRewardMaxTXB)
	if err != nil || dailyRewardMaxMinor < dailyRewardMinMinor {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_DAILY_REWARD_RANGE", "Daily reward maximum must be at least the minimum.")
		return
	}
	groupMessageRewardMinor, err := billing.ParseTXBMajor(request.GroupMessageRewardTXB)
	if err != nil || groupMessageRewardMinor < 0 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_GROUP_MESSAGE_REWARD", "Group message reward must be a non-negative TXB amount.")
		return
	}
	config, err := s.activityConfig(r.Context())
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	actorID := currentUser(r).ID
	if err := s.deps.Admin.PutSetting(r.Context(), actorID, activityTimezoneSetting, request.Timezone); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	if err := s.deps.Admin.PutSetting(r.Context(), actorID, groupMessageThresholdSetting, strconv.Itoa(request.GroupMessageThreshold)); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	if dailyRewardMaxMinor < config.RewardMinMinor {
		if err := s.deps.Admin.PutSetting(r.Context(), actorID, activityRewardMinSetting, request.DailyRewardMinTXB); err != nil {
			s.communityFailure(w, r, err)
			return
		}
		if err := s.deps.Admin.PutSetting(r.Context(), actorID, activityRewardMaxSetting, request.DailyRewardMaxTXB); err != nil {
			s.communityFailure(w, r, err)
			return
		}
	} else {
		if err := s.deps.Admin.PutSetting(r.Context(), actorID, activityRewardMaxSetting, request.DailyRewardMaxTXB); err != nil {
			s.communityFailure(w, r, err)
			return
		}
		if err := s.deps.Admin.PutSetting(r.Context(), actorID, activityRewardMinSetting, request.DailyRewardMinTXB); err != nil {
			s.communityFailure(w, r, err)
			return
		}
	}
	if err := s.deps.Admin.PutSetting(r.Context(), actorID, groupMessageRewardSetting, request.GroupMessageRewardTXB); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	config, err = s.activityConfig(r.Context())
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone": config.Timezone, "dailyRewardMinTxb": txbMajorString(config.RewardMinMinor),
		"dailyRewardMinTxbMinor": strconv.FormatInt(config.RewardMinMinor, 10), "dailyRewardMaxTxb": txbMajorString(config.RewardMaxMinor),
		"dailyRewardMaxTxbMinor": strconv.FormatInt(config.RewardMaxMinor, 10), "groupMessageThreshold": config.GroupMessageThreshold,
		"groupMessageRewardTxb": txbMajorString(config.GroupMessageRewardMinor), "groupMessageRewardTxbMinor": strconv.FormatInt(config.GroupMessageRewardMinor, 10),
	})
}

func (s *Server) adminActivityGames(w http.ResponseWriter, r *http.Request) {
	games, err := s.deps.Activity.Games(r.Context(), false)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	items := make([]activityGameResponse, 0, len(games))
	for _, game := range games {
		items = append(items, mapActivityGame(game))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type activityGameRequest struct {
	Name                string `json:"name"`
	Icon                string `json:"icon"`
	Description         string `json:"description"`
	Enabled             bool   `json:"enabled"`
	WinChanceBPS        int    `json:"winChanceBps"`
	MinimumStakeMinor   string `json:"minimumStakeMinor"`
	MaximumStakeMinor   string `json:"maximumStakeMinor"`
	ReturnMultiplierBPS int64  `json:"returnMultiplierBps"`
}

func (s *Server) adminCreateActivityGame(w http.ResponseWriter, r *http.Request) {
	s.adminSaveActivityGame(w, r, "")
}

func (s *Server) adminUpdateActivityGame(w http.ResponseWriter, r *http.Request) {
	s.adminSaveActivityGame(w, r, chiURLParam(r, "id"))
}

func (s *Server) adminDeleteActivityGame(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Store.DeleteActivityGame(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), time.Now().UTC()); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminActivityGameStatistics(w http.ResponseWriter, r *http.Request) {
	from, to, bucket, location, ok := s.statisticsWindow(w, r)
	if !ok {
		return
	}
	statistics, err := s.deps.Store.ActivityGameStatistics(r.Context(), chiURLParam(r, "id"), from, to, bucket, location)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, statistics)
}

func (s *Server) adminSaveActivityGame(w http.ResponseWriter, r *http.Request, id string) {
	var request activityGameRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ACTIVITY_GAME", "Game fields are invalid.")
		return
	}
	minimum, minimumErr := parseMinorString(request.MinimumStakeMinor, false)
	maximum, maximumErr := parseMinorString(request.MaximumStakeMinor, false)
	if minimumErr != nil || maximumErr != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_STAKE_RANGE", "Stake limits must be positive decimal integer strings in TXB hundredths.")
		return
	}
	game, err := s.deps.Activity.SaveGame(r.Context(), activity.GameInput{ID: id, Name: request.Name, Icon: request.Icon,
		Description: request.Description, Enabled: request.Enabled, WinChanceBPS: request.WinChanceBPS, MinimumStakeMinor: minimum,
		MaximumStakeMinor: maximum, ReturnMultiplierBPS: request.ReturnMultiplierBPS})
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, mapActivityGame(game))
}
