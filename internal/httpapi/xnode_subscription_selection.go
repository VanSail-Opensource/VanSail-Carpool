package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

func (s *Server) xnodePreviewSubscriptionSelection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Config    xnode.SubscriptionTemplateConfig `json:"config"`
		Format    string                           `json:"format"`
		UserAgent string                           `json:"userAgent"`
	}
	if decodeJSON(w, r, &input) != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	selection, validation := xnode.PreviewSubscriptionSelection(input.Config, input.Format, input.UserAgent)
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]any{"selection": selection, "validation": validation})
}
