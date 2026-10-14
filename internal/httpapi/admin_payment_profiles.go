package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/go-chi/chi/v5"
)

type paymentProfileLifecycle interface {
	RefreshPaymentProfile(context.Context, string) (model.PaymentProfile, error)
	DeletePaymentProfile(context.Context, string) error
}

func (s *Server) adminPaymentProfiles(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Settings.PaymentProfiles(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type paymentProfileWrite struct {
	ID              string   `json:"id"`
	Provider        string   `json:"provider"`
	ProviderName    string   `json:"providerName"`
	EnabledChannels []string `json:"enabledChannels"`
	Endpoint        string   `json:"endpoint"`
	MerchantID      string   `json:"merchantId"`
	Credential      string   `json:"credential"`
	Acknowledgement string   `json:"acknowledgement"`
	Enabled         bool     `json:"enabled"`
}

func (s *Server) adminCreatePaymentProfile(w http.ResponseWriter, r *http.Request) {
	s.adminSavePaymentProfileWithID(w, r, "")
}

func (s *Server) adminSavePaymentProfile(w http.ResponseWriter, r *http.Request) {
	s.adminSavePaymentProfileWithID(w, r, chi.URLParam(r, "id"))
}

func (s *Server) adminSavePaymentProfileWithID(w http.ResponseWriter, r *http.Request, id string) {
	created := id == ""
	var profile paymentProfileWrite
	if err := decodeJSON(w, r, &profile); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_PAYMENT_PROFILE", "Payment profile fields are invalid.")
		return
	}
	if id == "" {
		var err error
		id, err = ids.New()
		if err != nil {
			s.adminFailure(w, r, err)
			return
		}
	}
	saved, err := s.deps.Settings.SavePaymentProfile(r.Context(), currentUser(r).ID, model.PaymentProfile{
		ID: id, Provider: profile.Provider, ProviderName: profile.ProviderName, EnabledChannels: profile.EnabledChannels,
		Endpoint: profile.Endpoint, MerchantID: profile.MerchantID, Credential: profile.Credential,
		Acknowledgement: profile.Acknowledgement, Enabled: profile.Enabled,
	})
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	actorID := currentUser(r).ID
	_ = s.deps.Store.AppendAudit(r.Context(), &actorID, "payment_profile.update", "payment_profile", saved.ID, "{}", time.Now().UTC())
	if saved.Provider == "bepusdt" {
		saved, err = s.deps.PaymentProfiles.RefreshPaymentProfile(r.Context(), saved.ID)
		if err != nil {
			s.adminFailure(w, r, err)
			return
		}
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, saved)
}

func (s *Server) adminDeletePaymentProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.deps.PaymentProfiles.DeletePaymentProfile(r.Context(), id); err != nil {
		s.adminFailure(w, r, err)
		return
	}
	actorID := currentUser(r).ID
	_ = s.deps.Store.AppendAudit(r.Context(), &actorID, "payment_profile.delete", "payment_profile", id, "{}", time.Now().UTC())
	w.WriteHeader(http.StatusNoContent)
}
