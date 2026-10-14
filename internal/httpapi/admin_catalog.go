package httpapi

import (
	"net/http"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
)

func (s *Server) adminCombos(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Store.ListCombos(r.Context(), false)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type comboRequest struct {
	IPLookupQuota           *int     `json:"ipLookupQuota"`
	Name                    string   `json:"name"`
	Description             string   `json:"description"`
	PriceTXBMinor           string   `json:"priceTxbMinor"`
	ValidityDays            int      `json:"validityDays"`
	TrafficLimitBytes       string   `json:"trafficLimitBytes"`
	ResetStrategy           string   `json:"resetStrategy"`
	Active                  bool     `json:"active"`
	SquadProductIDs         []string `json:"squadProductIds"`
	IncludedSquadIDs        []string `json:"includedSquadIds"`
	RolloverMinRemainingBPS int      `json:"rolloverMinRemainingBps"`
}

func (s *Server) adminCreateCombo(w http.ResponseWriter, r *http.Request) {
	s.adminSaveCombo(w, r, "")
}

func (s *Server) adminUpdateCombo(w http.ResponseWriter, r *http.Request) {
	s.adminSaveCombo(w, r, chiURLParam(r, "id"))
}

func (s *Server) adminSaveCombo(w http.ResponseWriter, r *http.Request, id string) {
	var request comboRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COMBO", "Combo fields are invalid.")
		return
	}
	price, priceErr := strconv.ParseInt(request.PriceTXBMinor, 10, 64)
	traffic, trafficErr := strconv.ParseInt(request.TrafficLimitBytes, 10, 64)
	if priceErr != nil || trafficErr != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_COMBO", "Price and traffic must be decimal integer strings.")
		return
	}
	squadIDs := request.SquadProductIDs
	if len(squadIDs) == 0 {
		squadIDs = request.IncludedSquadIDs
	}
	combo, err := s.deps.Admin.SaveCombo(r.Context(), currentUser(r).ID, database.ComboInput{ID: id, Name: request.Name,
		Description: request.Description, PriceTXBMinor: price, ValidityDays: request.ValidityDays, TrafficLimitBytes: traffic,
		ResetStrategy: request.ResetStrategy, Active: request.Active, SquadProductIDs: squadIDs,
		RolloverMinRemainingBPS: request.RolloverMinRemainingBPS, IPLookupQuota: request.IPLookupQuota})
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, combo)
}

func (s *Server) adminDeleteCombo(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Admin.DeleteCombo(r.Context(), currentUser(r).ID, chiURLParam(r, "id")); err != nil {
		s.adminFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminComboStatistics(w http.ResponseWriter, r *http.Request) {
	from, to, bucket, location, ok := s.statisticsWindow(w, r)
	if !ok {
		return
	}
	statistics, err := s.deps.Store.ComboStatistics(r.Context(), chiURLParam(r, "id"), from, to, bucket, location)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, statistics)
}

func (s *Server) adminSquadProducts(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Admin.Squads(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type squadProductRequest struct {
	RemnaSquadUUID     string                `json:"remnaSquadUuid"`
	Name               string                `json:"name"`
	Description        string                `json:"description"`
	Profile            *squadprofile.Profile `json:"profile"`
	PriceTXBMinor      string                `json:"priceTxbMinor"`
	Visible            bool                  `json:"visible"`
	StockLimit         *int                  `json:"stockLimit"`
	ActivationRequired bool                  `json:"activationRequired"`
	ActivationCode     string                `json:"activationCode"`
	GeocheckEnabled    *bool                 `json:"geocheckEnabled"`
}

func (s *Server) adminCreateSquadProduct(w http.ResponseWriter, r *http.Request) {
	s.adminSaveSquadProduct(w, r, "")
}

func (s *Server) adminUpdateSquadProduct(w http.ResponseWriter, r *http.Request) {
	s.adminSaveSquadProduct(w, r, chiURLParam(r, "id"))
}

func (s *Server) adminSaveSquadProduct(w http.ResponseWriter, r *http.Request, id string) {
	var request squadProductRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_SQUAD_PRODUCT", "Squad product fields are invalid.")
		return
	}
	price, err := strconv.ParseInt(request.PriceTXBMinor, 10, 64)
	if err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_PRICE", "Price must be integer hundredths of TXB.")
		return
	}
	if request.RemnaSquadUUID == "" {
		request.RemnaSquadUUID = id
	}
	product, err := s.deps.Admin.SaveSquadProduct(r.Context(), currentUser(r).ID, database.SquadProductInput{ID: id,
		RemnaSquadUUID: request.RemnaSquadUUID, Name: request.Name, Description: request.Description, PriceTXBMinor: price,
		Profile: request.Profile, Visible: request.Visible, UpstreamPresent: true, StockLimit: request.StockLimit,
		ActivationRequired: request.ActivationRequired, ActivationCode: request.ActivationCode, GeocheckEnabled: request.GeocheckEnabled})
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, product)
}

func (s *Server) adminImportSquads(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Admin.ImportSquads(r.Context(), currentUser(r).ID)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminSquadStatistics(w http.ResponseWriter, r *http.Request) {
	from, to, bucket, location, ok := s.statisticsWindow(w, r)
	if !ok {
		return
	}
	statistics, err := s.deps.Store.SquadStatistics(r.Context(), chiURLParam(r, "id"), from, to, bucket, location)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, statistics)
}
