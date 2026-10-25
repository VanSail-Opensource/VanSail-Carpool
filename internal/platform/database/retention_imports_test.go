package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

func TestMaintenancePrunesAbandonedUploadsButKeepsLiveSettlement(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.Add(-8 * 24 * time.Hour)
	doc, err := questionnaires.ParseCSV(strings.NewReader("code\nunused\n"), 1024, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	for i, status := range []string{"preview", "failed", "queued", "processing"} {
		q, err := s.SaveQuestionnaire(ctx, questionnaires.QuestionnaireInput{Title: fmt.Sprint(i), FormURL: "https://forms.gle/abcdefgh", RewardTXBMinor: 1, Status: questionnaires.StatusClosed}, old)
		if err != nil {
			t.Fatal(err)
		}
		upload, err := s.CreateQuestionnaireImport(ctx, q.ID, doc, fmt.Sprint(i), old)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB().ExecContext(ctx, `UPDATE questionnaire_imports SET status=? WHERE id=?`, status, upload.ID); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := s.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil || counts["questionnaire_imports"] != 2 {
		t.Fatal(counts, err)
	}
	var live int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM questionnaire_imports WHERE status IN ('queued','processing') AND length(raw_csv)>0`).Scan(&live); err != nil || live != 2 {
		t.Fatal(live, err)
	}
}
