package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

func (s *Server) adminQuestionnaires(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Questionnaires.List(r.Context())
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	response := make([]questionnaireResponse, 0, len(items))
	for _, item := range items {
		response = append(response, mapQuestionnaire(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response})
}

type questionnaireRequest struct {
	Title          *string                         `json:"title"`
	Description    *string                         `json:"description"`
	FormURL        *string                         `json:"formUrl"`
	RewardTXBMinor *string                         `json:"rewardTxbMinor"`
	Status         *questionnaires.Status          `json:"status"`
	ClosesAt       nullableRequestField[time.Time] `json:"closesAt"`
}

func (s *Server) adminCreateQuestionnaire(w http.ResponseWriter, r *http.Request) {
	s.adminSaveQuestionnaire(w, r, "")
}

func (s *Server) adminUpdateQuestionnaire(w http.ResponseWriter, r *http.Request) {
	s.adminSaveQuestionnaire(w, r, chiURLParam(r, "id"))
}

func (s *Server) adminCloseQuestionnaire(w http.ResponseWriter, r *http.Request) {
	status := questionnaires.StatusClosed
	input, err := s.questionnaireInput(r.Context(), chiURLParam(r, "id"), questionnaireRequest{Status: &status})
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	if _, err := s.deps.Questionnaires.Save(r.Context(), input); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminDeleteQuestionnaire(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Store.DeleteQuestionnaire(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), time.Now().UTC()); err != nil {
		s.communityFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminSaveQuestionnaire(w http.ResponseWriter, r *http.Request, id string) {
	var request questionnaireRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_QUESTIONNAIRE", "Questionnaire fields are invalid.")
		return
	}
	input, err := s.questionnaireInput(r.Context(), id, request)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	item, err := s.deps.Questionnaires.Save(r.Context(), input)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, mapQuestionnaire(item))
}

func (s *Server) questionnaireInput(ctx context.Context, id string, request questionnaireRequest) (questionnaires.QuestionnaireInput, error) {
	input := questionnaires.QuestionnaireInput{ID: id, Status: questionnaires.StatusDraft}
	if id != "" {
		items, err := s.deps.Questionnaires.List(ctx)
		if err != nil {
			return questionnaires.QuestionnaireInput{}, err
		}
		found := false
		for _, item := range items {
			if item.ID == id {
				input = item.QuestionnaireInput
				found = true
				break
			}
		}
		if !found {
			return questionnaires.QuestionnaireInput{}, database.ErrNotFound
		}
	}
	input.ID = id
	if request.Title != nil {
		input.Title = *request.Title
	}
	if request.Description != nil {
		input.Description = *request.Description
	}
	if request.FormURL != nil {
		input.FormURL = *request.FormURL
	}
	if request.RewardTXBMinor != nil {
		value, err := parseMinorString(*request.RewardTXBMinor, true)
		if err != nil {
			return questionnaires.QuestionnaireInput{}, questionnaires.ErrInvalidInput
		}
		input.RewardTXBMinor = value
	}
	if request.Status != nil {
		input.Status = *request.Status
	}
	if request.ClosesAt.Set {
		input.ClosesAt = request.ClosesAt.Value
	}
	return input, nil
}

func (s *Server) adminActivateQuestionnaire(w http.ResponseWriter, r *http.Request) {
	id := chiURLParam(r, "id")
	input, err := s.questionnaireInput(r.Context(), id, questionnaireRequest{})
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	input.Status = questionnaires.StatusActive
	item, err := s.deps.Questionnaires.Save(r.Context(), input)
	if err != nil {
		s.communityFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapQuestionnaire(item))
}
