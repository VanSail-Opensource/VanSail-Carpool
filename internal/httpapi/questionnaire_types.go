package httpapi

import (
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

type questionnaireResponse struct {
	ID               string                `json:"id"`
	Title            string                `json:"title"`
	Description      string                `json:"description"`
	FormURL          string                `json:"formUrl"`
	RewardTXBMinor   string                `json:"rewardTxbMinor"`
	Status           questionnaires.Status `json:"status"`
	ClosesAt         *time.Time            `json:"closesAt"`
	ParticipantCount int                   `json:"participantCount"`
	RewardedCount    int                   `json:"rewardedCount"`
	CreatedAt        time.Time             `json:"createdAt"`
	UpdatedAt        time.Time             `json:"updatedAt"`
}

type questionnaireParticipationResponse struct {
	ID              string     `json:"id"`
	QuestionnaireID string     `json:"questionnaireId"`
	ValidationCode  string     `json:"validationCode"`
	AwardedAt       *time.Time `json:"awardedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type activeQuestionnaireResponse struct {
	ID             string                              `json:"id"`
	Title          string                              `json:"title"`
	Description    string                              `json:"description"`
	FormURL        string                              `json:"formUrl"`
	RewardTXBMinor string                              `json:"rewardTxbMinor"`
	ClosesAt       *time.Time                          `json:"closesAt"`
	Participation  *questionnaireParticipationResponse `json:"participation"`
}

type questionnaireHistoryResponse struct {
	Questionnaire questionnaireResponse              `json:"questionnaire"`
	Participation questionnaireParticipationResponse `json:"participation"`
}

type questionnaireSettlementResponse struct {
	ImportID            string    `json:"importId"`
	QuestionnaireID     string    `json:"questionnaireId"`
	MatchedCount        int       `json:"matchedCount"`
	DuplicateCount      int       `json:"duplicateCount"`
	UnknownCount        int       `json:"unknownCount"`
	MalformedCount      int       `json:"malformedCount"`
	AlreadyAwardedCount int       `json:"alreadyAwardedCount"`
	RewardedCount       int       `json:"rewardedCount"`
	RewardTXBMinor      string    `json:"rewardTxbMinor"`
	SettledAt           time.Time `json:"settledAt"`
	Replayed            bool      `json:"replayed"`
}

type questionnaireImportStateResponse struct {
	Preview questionnaires.ImportPreview     `json:"preview"`
	Report  *questionnaireSettlementResponse `json:"report,omitempty"`
}

func mapQuestionnaireImportState(state questionnaires.ImportState) questionnaireImportStateResponse {
	response := questionnaireImportStateResponse{Preview: mapQuestionnaireImportPreview(state.Preview)}
	if state.Report != nil {
		report := state.Report
		response.Report = &questionnaireSettlementResponse{
			ImportID: report.ImportID, QuestionnaireID: report.QuestionnaireID, MatchedCount: report.MatchedCount,
			DuplicateCount: report.DuplicateCount, UnknownCount: report.UnknownCount, MalformedCount: report.MalformedCount,
			AlreadyAwardedCount: report.AlreadyAwardedCount, RewardedCount: report.RewardedCount,
			RewardTXBMinor: strconv.FormatInt(report.RewardTXBMinor, 10), SettledAt: report.SettledAt, Replayed: report.Replayed,
		}
	}
	return response
}

func mapQuestionnaireImportPreview(preview questionnaires.ImportPreview) questionnaires.ImportPreview {
	if preview.Headers == nil {
		preview.Headers = []string{}
	}
	if preview.SampleRows == nil {
		preview.SampleRows = [][]string{}
	}
	return preview
}

func mapQuestionnaire(item questionnaires.Questionnaire) questionnaireResponse {
	return questionnaireResponse{ID: item.ID, Title: item.Title, Description: item.Description, FormURL: item.FormURL,
		RewardTXBMinor: strconv.FormatInt(item.RewardTXBMinor, 10), Status: item.Status, ClosesAt: item.ClosesAt,
		ParticipantCount: item.ParticipantCount, RewardedCount: item.RewardedCount, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapQuestionnaireParticipation(item questionnaires.Participant) questionnaireParticipationResponse {
	return questionnaireParticipationResponse{ID: item.ID, QuestionnaireID: item.QuestionnaireID,
		ValidationCode: item.ValidationCode, AwardedAt: item.AwardedAt, CreatedAt: item.CreatedAt}
}
