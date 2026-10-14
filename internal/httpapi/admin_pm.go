package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type pmConversationResponse struct {
	ProfileState     string    `json:"profileState"`
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	TelegramID       string    `json:"telegramId"`
	Name             string    `json:"name"`
	Username         string    `json:"username"`
	ChatID           string    `json:"chatId"`
	TopicID          *string   `json:"topicId"`
	ProfileMessageID *string   `json:"profileMessageId"`
	TopicState       string    `json:"topicState"`
	Blocked          bool      `json:"blocked"`
	Muted            bool      `json:"muted"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func optionalPMID(value int64) *string {
	if value <= 0 {
		return nil
	}
	formatted := strconv.FormatInt(value, 10)
	return &formatted
}
func mapPMConversation(item model.PMConversation) pmConversationResponse {
	name := strings.TrimSpace(item.FirstName + " " + item.LastName)
	if name == "" {
		name = strconv.FormatInt(item.TelegramID, 10)
	}
	return pmConversationResponse{ID: item.ID, UserID: item.UserID, TelegramID: strconv.FormatInt(item.TelegramID, 10), Name: name, Username: item.Username,
		ChatID: strconv.FormatInt(item.ChatID, 10), TopicID: optionalPMID(item.TopicID), ProfileMessageID: optionalPMID(item.ProfileMessageID),
		TopicState: item.TopicState, ProfileState: item.ProfileState, Blocked: item.Blocked, Muted: item.Muted, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func (s *Server) adminPMConversations(w http.ResponseWriter, r *http.Request) {
	query, ok := s.parseAdminInventoryQuery(w, r, []string{})
	if !ok {
		return
	}
	items, cursor, err := s.deps.Store.ListPMConversations(r.Context(), query.Cursor, query.Search, query.Limit)
	if err != nil {
		s.writeAdminPageFailure(w, r, err)
		return
	}
	response := make([]pmConversationResponse, len(items))
	for index, item := range items {
		response[index] = mapPMConversation(item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response, "page": map[string]any{"nextCursor": cursor}})
}

func (s *Server) adminPMModeration(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input struct {
		Blocked *bool `json:"blocked"`
		Muted   *bool `json:"muted"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.Blocked == nil && input.Muted == nil {
		s.writeError(w, r, 422, "INVALID_PM_UPDATE", "A block or mute preference is required.")
		return
	}
	receipt, err := s.deps.Store.QueuePMModeration(r.Context(), currentUser(r).ID, key, model.PMModerationInput{ConversationID: chiURLParam(r, "id"), Blocked: input.Blocked, Muted: input.Muted}, 0, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) adminPMRepairTopic(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		TopicID          string `json:"topicId"`
		ProfileMessageID string `json:"profileMessageId"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, 422, "INVALID_PM_TOPIC", "Valid Telegram topic and profile references are required.")
		return
	}
	topic, err := strconv.ParseInt(request.TopicID, 10, 64)
	profile := int64(0)
	var profileErr error
	if request.ProfileMessageID != "" {
		profile, profileErr = strconv.ParseInt(request.ProfileMessageID, 10, 64)
	}
	if err != nil || profileErr != nil || topic <= 1 || profile < 0 {
		s.writeError(w, r, 422, "INVALID_PM_TOPIC", "Valid Telegram topic and profile references are required.")
		return
	}
	receipt, err := s.deps.Store.QueuePMTopicRepair(r.Context(), currentUser(r).ID, key, model.PMTopicRepairInput{ConversationID: chiURLParam(r, "id"), TopicID: topic, ProfileMessageID: profile}, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}
