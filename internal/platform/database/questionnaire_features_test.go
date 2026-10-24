package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

type sequenceQuestionnaireCodes struct {
	codes []string
	next  int
}

func (generator *sequenceQuestionnaireCodes) NewCode() (string, error) {
	code := generator.codes[generator.next]
	generator.next++
	return code, nil
}

func TestQuestionnaireSingleActiveAndIdempotentCSVSettlement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 33_000)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	questionnaire, err := store.SaveQuestionnaire(ctx, questionnaires.QuestionnaireInput{Title: "Feedback", FormURL: "https://forms.gle/abcdefgh",
		RewardTXBMinor: 250, Status: questionnaires.StatusActive}, now)
	if err != nil {
		t.Fatalf("SaveQuestionnaire(): %v", err)
	}
	if _, err := store.SaveQuestionnaire(ctx, questionnaires.QuestionnaireInput{Title: "Second", FormURL: "https://forms.office.com/r/example",
		RewardTXBMinor: 1, Status: questionnaires.StatusActive}, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("second active questionnaire error = %v, want conflict", err)
	}
	generator := &sequenceQuestionnaireCodes{codes: []string{"txq-known"}}
	participant, err := store.EnsureQuestionnaireParticipant(ctx, questionnaire.ID, user.ID, generator, now)
	if err != nil {
		t.Fatalf("EnsureQuestionnaireParticipant(): %v", err)
	}
	retrieved, err := store.EnsureQuestionnaireParticipant(ctx, questionnaire.ID, user.ID, generator, now.Add(time.Minute))
	if err != nil || retrieved.ID != participant.ID || retrieved.ValidationCode != "TXQ-KNOWN" {
		t.Fatalf("participant replay = (%+v, %v)", retrieved, err)
	}
	raw := "code,name\nTXQ-KNOWN,Alice\ntxq-known,Duplicate\nTXQ-UNKNOWN,Unknown\nmissing-column\n"
	document, err := questionnaires.ParseCSV(strings.NewReader(raw), 5<<20, 50_000, 100)
	if err != nil {
		t.Fatalf("ParseCSV(): %v", err)
	}
	preview, err := store.CreateQuestionnaireImport(ctx, questionnaire.ID, document, "upload-key", now)
	if err != nil {
		t.Fatalf("CreateQuestionnaireImport(): %v", err)
	}
	analysis, err := store.AnalyzeQuestionnaireImport(ctx, preview.ID, "code", now)
	if err != nil {
		t.Fatalf("AnalyzeQuestionnaireImport(): %v", err)
	}
	if analysis.MatchedCount != 1 || analysis.DuplicateCount != 1 || analysis.UnknownCount != 1 || analysis.MalformedCount != 1 {
		t.Fatalf("import analysis = %+v", analysis)
	}
	queued, err := store.QueueQuestionnaireSettlement(ctx, preview.ID, now)
	if err != nil || queued.Status != questionnaires.ImportStatusQueued {
		t.Fatalf("QueueQuestionnaireSettlement() = (%+v, %v)", queued, err)
	}
	report, err := store.SettleQuestionnaireImport(ctx, preview.ID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SettleQuestionnaireImport(): %v", err)
	}
	if report.MatchedCount != 1 || report.DuplicateCount != 1 || report.UnknownCount != 1 || report.MalformedCount != 1 || report.RewardedCount != 1 {
		t.Fatalf("settlement report = %+v", report)
	}
	replayed, err := store.SettleQuestionnaireImport(ctx, preview.ID, now.Add(2*time.Minute))
	if err != nil || !replayed.Replayed || replayed.RewardedCount != 1 {
		t.Fatalf("settlement replay = (%+v, %v)", replayed, err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "250" {
		t.Fatalf("Balance() = (%+v, %v), want 250", balance, err)
	}
	state, err := store.QuestionnaireImportState(ctx, preview.ID)
	if err != nil || state.Report == nil || state.Preview.Status != questionnaires.ImportStatusSettled {
		t.Fatalf("QuestionnaireImportState() = (%+v, %v)", state, err)
	}
	history, err := store.ListQuestionnaireParticipations(ctx, user.ID, 10)
	if err != nil || len(history) != 1 || history[0].Participation.AwardedAt == nil {
		t.Fatalf("ListQuestionnaireParticipations() = (%+v, %v)", history, err)
	}
	if history[0].Questionnaire.ParticipantCount != 1 || history[0].Questionnaire.RewardedCount != 1 {
		t.Fatalf("participation history counters = (%d, %d), want (1, 1)", history[0].Questionnaire.ParticipantCount, history[0].Questionnaire.RewardedCount)
	}
	items, err := store.ListQuestionnaires(ctx)
	if err != nil {
		t.Fatalf("ListQuestionnaires(): %v", err)
	}
	if len(items) != 1 || items[0].ParticipantCount != 1 || items[0].RewardedCount != 1 {
		t.Fatalf("questionnaire counters = %+v, want one participant and one rewarded participant", items)
	}
	var jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='questionnaire_settlement' AND payload=?`, `{"importId":"`+preview.ID+`"}`).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("questionnaire outbox jobs = %d, %v, want 1", jobs, err)
	}
}

func TestQuestionnaireExpiryHidesFormButPreservesIssuedCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	participantUser := createTestUser(t, store, 33_100)
	lateUser := createTestUser(t, store, 33_101)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	closesAt := now.Add(time.Hour)
	questionnaire, err := store.SaveQuestionnaire(ctx, questionnaires.QuestionnaireInput{Title: "Timed feedback", Description: "  Before the deadline  ",
		FormURL: "https://forms.gle/abcdefgh", RewardTXBMinor: 25, Status: questionnaires.StatusActive, ClosesAt: &closesAt}, now)
	if err != nil {
		t.Fatalf("SaveQuestionnaire(): %v", err)
	}
	if questionnaire.ClosesAt == nil || !questionnaire.ClosesAt.Equal(closesAt) || questionnaire.Description != "Before the deadline" {
		t.Fatalf("saved questionnaire = %+v", questionnaire)
	}
	active, err := store.ActiveQuestionnaire(ctx, closesAt.Add(-time.Nanosecond))
	if err != nil || active == nil || active.ID != questionnaire.ID {
		t.Fatalf("ActiveQuestionnaire(before close) = (%+v, %v)", active, err)
	}
	generator := &sequenceQuestionnaireCodes{codes: []string{"txq-durable"}}
	issued, err := store.EnsureQuestionnaireParticipant(ctx, questionnaire.ID, participantUser.ID, generator, closesAt.Add(-time.Minute))
	if err != nil {
		t.Fatalf("EnsureQuestionnaireParticipant(before close): %v", err)
	}
	active, err = store.ActiveQuestionnaire(ctx, closesAt)
	if err != nil || active != nil {
		t.Fatalf("ActiveQuestionnaire(at close) = (%+v, %v), want nil", active, err)
	}
	retrieved, err := store.EnsureQuestionnaireParticipant(ctx, questionnaire.ID, participantUser.ID, generator, closesAt)
	if err != nil || retrieved.ID != issued.ID || retrieved.ValidationCode != issued.ValidationCode {
		t.Fatalf("existing code at close = (%+v, %v)", retrieved, err)
	}
	if _, err := store.EnsureQuestionnaireParticipant(ctx, questionnaire.ID, lateUser.ID, generator, closesAt); !errors.Is(err, ErrConflict) {
		t.Fatalf("new participant at close error = %v, want conflict", err)
	}
	if generator.next != 1 {
		t.Fatalf("code generator calls = %d, want 1", generator.next)
	}
	history, err := store.ListQuestionnaireParticipations(ctx, participantUser.ID, 10)
	if err != nil || len(history) != 1 || history[0].Questionnaire.ClosesAt == nil || !history[0].Questionnaire.ClosesAt.Equal(closesAt) {
		t.Fatalf("participation history = (%+v, %v)", history, err)
	}

	next, err := store.SaveQuestionnaire(ctx, questionnaires.QuestionnaireInput{Title: "Next feedback", FormURL: "https://forms.office.com/r/example",
		Status: questionnaires.StatusActive}, closesAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("activate after expired questionnaire: %v", err)
	}
	if next.ID == questionnaire.ID {
		t.Fatal("next questionnaire reused the expired questionnaire ID")
	}
	items, err := store.ListQuestionnaires(ctx)
	if err != nil {
		t.Fatalf("ListQuestionnaires(): %v", err)
	}
	for _, item := range items {
		if item.ID == questionnaire.ID && item.Status != questionnaires.StatusClosed {
			t.Fatalf("expired questionnaire status = %q, want closed", item.Status)
		}
	}
}

func TestQuestionnaireRejectsAlreadyExpiredActiveSave(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	closesAt := now
	_, err := store.SaveQuestionnaire(ctx, questionnaires.QuestionnaireInput{Title: "Expired", FormURL: "https://forms.gle/abcdefgh",
		Status: questionnaires.StatusActive, ClosesAt: &closesAt}, now)
	if !errors.Is(err, questionnaires.ErrInvalidInput) {
		t.Fatalf("SaveQuestionnaire() error = %v, want invalid input", err)
	}
}

func TestQuestionnaireSettlementRollsBackIfLifecycleFinalizationConflicts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 33_200)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	questionnaire, err := store.SaveQuestionnaire(ctx, questionnaires.QuestionnaireInput{
		Title: "Atomic settlement", FormURL: "https://forms.gle/abcdefgh", RewardTXBMinor: 250, Status: questionnaires.StatusActive,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	participant, err := store.EnsureQuestionnaireParticipant(ctx, questionnaire.ID, user.ID, &sequenceQuestionnaireCodes{codes: []string{"TXQ-ATOMIC"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	document, err := questionnaires.ParseCSV(strings.NewReader("code\nTXQ-ATOMIC\n"), 5<<20, 50_000, 100)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.CreateQuestionnaireImport(ctx, questionnaire.ID, document, "atomic-conflict", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AnalyzeQuestionnaireImport(ctx, preview.ID, "code", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.QueueQuestionnaireSettlement(ctx, preview.ID, now); err != nil {
		t.Fatal(err)
	}
	conflictTrigger := `CREATE TRIGGER questionnaire_settlement_conflict
		AFTER UPDATE OF status ON questionnaire_imports WHEN NEW.status='settled'
		BEGIN UPDATE questionnaires SET status='closed' WHERE id=NEW.questionnaire_id; END`
	if IsPostgres(store.DB()) {
		conflictTrigger = `CREATE FUNCTION questionnaire_settlement_conflict_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN UPDATE questionnaires SET status='closed' WHERE id=NEW.questionnaire_id; RETURN NEW; END $$; CREATE TRIGGER questionnaire_settlement_conflict AFTER UPDATE OF status ON questionnaire_imports FOR EACH ROW WHEN (NEW.status='settled') EXECUTE FUNCTION questionnaire_settlement_conflict_fn()`
	}
	if _, err := store.DB().ExecContext(ctx, conflictTrigger); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SettleQuestionnaireImport(ctx, preview.ID, now.Add(time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("SettleQuestionnaireImport() error = %v, want ErrConflict", err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "0" {
		t.Fatalf("balance after rollback = (%+v, %v), want zero", balance, err)
	}
	stored, err := store.questionnaireParticipantByID(ctx, participant.ID)
	if err != nil || stored.AwardedAt != nil {
		t.Fatalf("participant after rollback = (%+v, %v)", stored, err)
	}
	state, err := store.QuestionnaireImportState(ctx, preview.ID)
	if err != nil || state.Preview.Status != questionnaires.ImportStatusQueued {
		t.Fatalf("import after rollback = (%+v, %v)", state, err)
	}
}
