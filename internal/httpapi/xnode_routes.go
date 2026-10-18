package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
)

func (s *Server) mountXNodeAdmin(router chi.Router) {
	if s.deps.XNode == nil {
		return
	}
	router.Get("/xnode/global", s.xnodeGlobal)
	router.Get("/xnode/routing-templates", s.xnodeRoutingTemplateConfig)
	router.Put("/xnode/routing-templates", s.xnodeSaveRoutingTemplateConfig)
	router.Get("/xnode/firewall/bans", s.xnodeFirewallBans)
	router.Post("/xnode/firewall/exemptions", s.xnodeGrantFirewallExemption)
	router.Get("/xnode/subscription-templates", s.xnodeSubscriptionTemplates)
	router.Put("/xnode/subscription-templates", s.xnodeSaveSubscriptionTemplates)
	router.Post("/xnode/subscription-templates/preview", s.xnodePreviewSubscriptionTemplate)
	router.Post("/xnode/subscription-templates/selection-preview", s.xnodePreviewSubscriptionSelection)
	router.Put("/xnode/global", s.xnodeSaveGlobal)
	router.Get("/xnode/nodes", s.xnodeNodes)
	router.Post("/xnode/nodes", s.xnodeSaveNode)
	router.Put("/xnode/nodes/{id}", s.xnodeSaveNode)
	router.Get("/xnode/nodes/{id}", s.xnodeNode)
	router.Delete("/xnode/nodes/{id}", s.xnodeDeleteNode)
	router.Post("/xnode/nodes/{id}/commands", s.xnodeQueueCommand)
	router.Get("/xnode/operations/{id}", s.xnodeCommandResult)
	router.Post("/xnode/nodes/{id}/enrollment", s.xnodeEnrollment)
	router.Get("/xnode/configurations", s.xnodeConfigurations)
	router.Post("/xnode/configurations", s.xnodeSaveConfiguration)
	router.Put("/xnode/configurations/{id}", s.xnodeSaveConfiguration)
	router.Delete("/xnode/configurations/{id}", s.xnodeDeleteConfiguration)
	router.Post("/xnode/configurations/validate", s.xnodeValidateConfiguration)
	router.Get("/xnode/snippets", s.xnodeSnippets)
	router.Post("/xnode/snippets", s.xnodeSaveSnippet)
	router.Put("/xnode/snippets/{id}", s.xnodeSaveSnippet)
	router.Delete("/xnode/snippets/{id}", s.xnodeDeleteSnippet)
	router.Get("/xnode/groups", s.xnodeGroups)
	router.Post("/xnode/groups", s.xnodeSaveGroup)
	router.Put("/xnode/groups/{id}", s.xnodeSaveGroup)
	router.Delete("/xnode/groups/{id}", s.xnodeDeleteGroup)
	router.Get("/xnode/endpoints", s.xnodeEndpoints)
	router.Post("/xnode/endpoints", s.xnodeSaveEndpoint)
	router.Put("/xnode/endpoints/{id}", s.xnodeSaveEndpoint)
	router.Delete("/xnode/endpoints/{id}", s.xnodeDeleteEndpoint)
	router.Get("/xnode/metrics", s.xnodeMetrics)
	router.Get("/xnode/service-identities/{id}/subscription", s.xnodeServiceSubscription)
	router.Get("/xnode/service-identities", s.xnodeNamedServices)
	router.Post("/xnode/service-identities", s.xnodeSaveNamedService)
	router.Put("/xnode/service-identities/{id}", s.xnodeSaveNamedService)
	router.Post("/xnode/service-identities/{id}/subscription/rotate", s.xnodeRotateServiceSubscription)
	router.Get("/xnode/service-identities/{id}/routing/{nodeId}", s.xnodeServiceRouting)
	router.Put("/xnode/service-identities/{id}/routing/{nodeId}", s.xnodeSaveServiceRouting)
	if s.deps.XNodeMigration != nil {
		router.Get("/xnode/migration", s.xnodeMigrationStatus)
		router.Post("/xnode/migration/preview", s.xnodeMigrationPreview)
		router.Post("/xnode/migration/import", s.xnodeMigrationImport)
		router.Post("/xnode/migration/final-export", s.xnodeMigrationFinalExport)
		router.Post("/xnode/migration/finalize", s.xnodeMigrationFinalize)
		router.Post("/xnode/migration/nodes/{id}/cutover", s.xnodeMigrationCutover)
		router.Post("/xnode/migration/nodes/{id}/rollback", s.xnodeMigrationRollback)
	}
}
func (s *Server) xnodeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusServiceUnavailable, "XNODE_UNAVAILABLE"
	var policy *xnode.PolicyError
	var capability *xnode.CapabilityError
	switch {
	case errors.As(err, &capability):
		status, code = http.StatusConflict, "XNODE_UPGRADE_REQUIRED"
	case errors.As(err, &policy):
		code = policy.Code
		if errors.Is(err, xnode.ErrInvalid) {
			status = http.StatusUnprocessableEntity
		}
	case errors.Is(err, xnode.ErrNotFound):
		status, code = http.StatusNotFound, "XNODE_NOT_FOUND"
	case errors.Is(err, xnode.ErrInvalid):
		status, code = http.StatusUnprocessableEntity, "XNODE_INPUT_INVALID"
	case errors.Is(err, xnode.ErrConflict):
		status, code = http.StatusConflict, "XNODE_CONFLICT"
	}
	s.writeError(w, r, status, code, "The node action could not be completed.")
}
func (s *Server) xnodeGlobal(w http.ResponseWriter, r *http.Request) {
	config, err := s.deps.XNode.Global(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	for name, provider := range config.Providers {
		for key := range provider.Credentials {
			provider.Credentials[key] = ""
		}
		config.Providers[name] = provider
	}
	writeJSON(w, http.StatusOK, config)
}
func (s *Server) xnodeSaveGlobal(w http.ResponseWriter, r *http.Request) {
	var config xnode.GlobalConfig
	if err := decodeJSON(w, r, &config); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if err := s.deps.XNode.SaveGlobal(r.Context(), currentUser(r).ID, config); err != nil {
		s.xnodeError(w, r, err)
		return
	}
	s.xnodeGlobal(w, r)
}
func (s *Server) xnodeMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.deps.XNodeMaster.Metrics(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, metrics)
}
