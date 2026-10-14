package httpapi

import (
	"bytes"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

func (s *Server) adminUploadQuestionnaireCSV(w http.ResponseWriter, r *http.Request) {
	key, err := optionalOrGeneratedIdempotencyKey(w, r)
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must contain 1 to 128 characters.")
		return
	}
	content, err := boundedMultipartFile(w, r, "file", maxQuestionnaireCSV)
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_CSV_UPLOAD", "Upload one UTF-8 CSV file no larger than 5 MiB.")
		return
	}
	preview, err := s.deps.Questionnaires.UploadCSV(r.Context(), chiURLParam(r, "id"), bytes.NewReader(content), key)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapQuestionnaireImportPreview(preview))
}

func (s *Server) adminAnalyzeQuestionnaireCSV(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CodeColumn string `json:"codeColumn"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_IMPORT_ANALYSIS", "Select one validation-code column.")
		return
	}
	if !s.questionnaireImportBelongs(w, r) {
		return
	}
	analysis, err := s.deps.Questionnaires.AnalyzeImport(r.Context(), chiURLParam(r, "importID"), request.CodeColumn)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, analysis)
}

func (s *Server) adminSettleQuestionnaireCSV(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	if !s.questionnaireImportBelongs(w, r) {
		return
	}
	receipt, err := s.deps.Questionnaires.ConfirmImportOperation(r.Context(), currentUser(r).ID,
		chiURLParam(r, "importID"), key)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	state, err := s.deps.Questionnaires.Import(r.Context(), chiURLParam(r, "importID"))
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"operation": receipt, "import": mapQuestionnaireImportPreview(state.Preview),
	})
}

func (s *Server) adminQuestionnaireImport(w http.ResponseWriter, r *http.Request) {
	state, ok := s.loadQuestionnaireImport(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, mapQuestionnaireImportState(state))
}

func (s *Server) questionnaireImportBelongs(w http.ResponseWriter, r *http.Request) bool {
	_, ok := s.loadQuestionnaireImport(w, r)
	return ok
}

func (s *Server) loadQuestionnaireImport(w http.ResponseWriter, r *http.Request) (questionnaires.ImportState, bool) {
	state, err := s.deps.Questionnaires.Import(r.Context(), chiURLParam(r, "importID"))
	if err != nil {
		s.communityFailure(w, r, err)
		return questionnaires.ImportState{}, false
	}
	if state.Preview.QuestionnaireID != chiURLParam(r, "id") {
		s.writeError(w, r, http.StatusNotFound, "IMPORT_NOT_FOUND", "Questionnaire import not found.")
		return questionnaires.ImportState{}, false
	}
	return state, true
}
