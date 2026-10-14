package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/currencydisplay"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type displayCurrencyRatesResponse struct {
	CNYTXBPerUnit *string `json:"cnyTxbPerUnit"`
	USDTXBPerUnit *string `json:"usdTxbPerUnit"`
}

type displayCurrencyResponse struct {
	Currency string                       `json:"currency"`
	Rates    displayCurrencyRatesResponse `json:"rates"`
}

type displayCurrencyRequest struct {
	Currency *string `json:"currency"`
}

func (s *Server) displayCurrency(w http.ResponseWriter, r *http.Request) {
	response, err := s.displayCurrencySnapshot(r.Context(), currentUser(r))
	if err != nil {
		s.writeError(w, r, http.StatusServiceUnavailable, "DISPLAY_CURRENCY_UNAVAILABLE", "Display currency settings are temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) updateDisplayCurrency(w http.ResponseWriter, r *http.Request) {
	var request displayCurrencyRequest
	if err := decodeJSON(w, r, &request); err != nil || request.Currency == nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_DISPLAY_CURRENCY", "Provide a supported display currency.")
		return
	}
	target, ok := currencydisplay.ParseCurrency(*request.Currency)
	if !ok {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_DISPLAY_CURRENCY", "Provide a supported display currency.")
		return
	}
	user := currentUser(r)
	response, err := s.displayCurrencySnapshot(r.Context(), user)
	if err != nil {
		s.writeError(w, r, http.StatusServiceUnavailable, "DISPLAY_CURRENCY_UNAVAILABLE", "Display currency settings are temporarily unavailable.")
		return
	}
	if target == currencydisplay.CNY && response.Rates.CNYTXBPerUnit == nil || target == currencydisplay.USD && response.Rates.USDTXBPerUnit == nil {
		s.writeError(w, r, http.StatusConflict, "DISPLAY_CURRENCY_RATE_UNAVAILABLE", "The requested display currency rate is unavailable.")
		return
	}
	updated, err := s.deps.Store.SetDisplayCurrency(r.Context(), user.ID, string(target))
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "DISPLAY_CURRENCY_SAVE_FAILED", "The display currency could not be saved.")
		return
	}
	response.Currency = displayCurrencyFor(updated)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) displayCurrencySnapshot(rctx context.Context, user model.User) (displayCurrencyResponse, error) {
	cny, err := s.displayCurrencyRate(rctx, "billing.rate.txb_per_cny")
	if err != nil {
		return displayCurrencyResponse{}, err
	}
	usd, err := s.displayCurrencyRate(rctx, "billing.rate.txb_per_usd")
	if err != nil {
		return displayCurrencyResponse{}, err
	}
	return displayCurrencyResponse{
		Currency: displayCurrencyFor(user),
		Rates:    displayCurrencyRatesResponse{CNYTXBPerUnit: cny, USDTXBPerUnit: usd},
	}, nil
}

func (s *Server) displayCurrencyRate(ctx context.Context, key string) (*string, error) {
	raw, err := s.deps.Settings.Optional(ctx, key)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if !currencydisplay.ValidRate(raw) {
		return nil, nil
	}
	return &raw, nil
}

func displayCurrencyFor(user model.User) string {
	currency, ok := currencydisplay.ParseCurrency(user.DisplayCurrency)
	if !ok {
		currency = currencydisplay.TXB
	}
	return string(currency)
}
