package httpapi

import "net/http"

func (s *Server) browserLink(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ReturnTo string `json:"returnTo"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.browserFailure(w, r, 400, "INVALID_REQUEST")
		return
	}
	link, expires, err := s.deps.BrowserAuth.Link(r.Context(), currentUser(r), input.ReturnTo)
	if err != nil {
		s.browserFailure(w, r, 422, "BROWSER_LINK_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"url": link, "expiresAt": expires})
}
