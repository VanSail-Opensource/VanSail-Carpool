package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 100 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_SEARCH", "Search must contain at most 100 characters.")
		return
	}
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 100 {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_PAGE_SIZE", "Page size must be between 1 and 100.")
			return
		}
		limit = parsed
	}
	filter := database.AdminUserSearchFilter{Search: search, State: r.URL.Query().Get("state"),
		ComboIDs: r.URL.Query()["comboId"], SquadUUIDs: r.URL.Query()["squadUuid"], Match: r.URL.Query().Get("match")}
	users, nextCursor, err := s.deps.Store.ListAdminUsersPage(r.Context(), r.URL.Query().Get("cursor"), filter, limit)
	if err != nil {
		if errors.Is(err, database.ErrInvalidAdminUserSearch) {
			s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_USER_FILTER", "The user search filter is invalid.")
			return
		}
		if errors.Is(err, database.ErrInvalidCursor) {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "The pagination cursor is invalid.")
			return
		}
		s.adminFailure(w, r, err)
		return
	}
	type synchronization struct {
		Status string `json:"status"`
	}
	type adminUser struct {
		User            userResponse    `json:"user"`
		Balance         model.Money     `json:"balance"`
		CreatedAt       time.Time       `json:"createdAt"`
		Synchronization synchronization `json:"synchronization"`
	}
	items := make([]adminUser, 0, len(users))
	for _, record := range users {
		user := record.User
		status := "not_provisioned"
		if user.ProxyUserID != nil {
			status = "synchronized"
		}
		items = append(items, adminUser{User: mapUser(user), Balance: record.Balance, CreatedAt: user.CreatedAt, Synchronization: synchronization{Status: status}})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": map[string]any{"nextCursor": nextCursor}})
}

func (s *Server) adminUser(w http.ResponseWriter, r *http.Request) {
	userID := chiURLParam(r, "id")
	detail, err := s.deps.AdminUsers.UserDetail(r.Context(), userID)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	blocks, err := s.deps.ConnectionDrops.List(r.Context(), userID)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	response := mapAdminUserDetail(detail)
	response.IPBlocks = blocks
	conversation, found, err := s.deps.Store.PMConversationByUser(r.Context(), userID)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	if found {
		mapped := mapPMConversation(conversation)
		response.PMConversation = &mapped
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) adminUnblockIP(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.ConnectionDrops.Unblock(r.Context(), currentUser(r).ID,
		chiURLParam(r, "userId"), chiURLParam(r, "blockId"), key)
	if err != nil {
		s.writeIPBlockError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) adminBalanceAdjustment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DeltaTXBMinor string `json:"deltaTxbMinor"`
		Reason        string `json:"reason"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_ADJUSTMENT", "Amount and reason are required.")
		return
	}
	delta, err := strconv.ParseInt(request.DeltaTXBMinor, 10, 64)
	if err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_AMOUNT", "Adjustment must be integer hundredths of TXB.")
		return
	}
	entry, err := s.deps.Admin.AdjustBalance(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), delta, request.Reason)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) adminEntitlements(w http.ResponseWriter, r *http.Request) {
	query, ok := s.parseAdminInventoryQuery(w, r, entitlementPageStatuses)
	if !ok {
		return
	}
	items, nextCursor, err := s.deps.Store.ListAdminPurchasesPage(r.Context(), query.Cursor, query.Search, query.Status, query.Limit)
	if err != nil {
		s.writeAdminPageFailure(w, r, err)
		return
	}
	type adminEntitlement struct {
		model.Purchase
		UserID string `json:"userId"`
	}
	response := make([]adminEntitlement, 0, len(items))
	for _, item := range items {
		response = append(response, adminEntitlement{Purchase: item, UserID: item.UserID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response, "page": map[string]any{"nextCursor": nextCursor}})
}

func (s *Server) adminCancelEntitlement(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_CANCELLATION", "A reason is required.")
		return
	}
	purchase, err := s.deps.Admin.CancelEntitlement(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), request.Reason)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, purchase)
}
