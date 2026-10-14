package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

func TestParseMinorString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		allowZero bool
		want      int64
		wantErr   bool
	}{
		{name: "positive", value: "15000", want: 15000},
		{name: "zero allowed", value: "0", allowZero: true},
		{name: "zero rejected", value: "0", wantErr: true},
		{name: "negative rejected", value: "-1", wantErr: true},
		{name: "leading plus rejected", value: "+1", wantErr: true},
		{name: "fraction rejected", value: "1.00", wantErr: true},
		{name: "space rejected", value: " 1", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseMinorString(test.value, test.allowZero)
			if test.wantErr && err == nil {
				t.Fatal("parseMinorString() unexpectedly succeeded")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("parseMinorString(): %v", err)
			}
			if got != test.want {
				t.Fatalf("parseMinorString() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestRequireIdempotencyKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		wantOK     bool
		wantStatus int
	}{
		{name: "present", value: "request-123", wantOK: true, wantStatus: http.StatusOK},
		{name: "missing", wantStatus: http.StatusBadRequest},
		{name: "too long", value: strings.Repeat("x", 129), wantStatus: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/", nil)
			request.Header.Set("Idempotency-Key", test.value)
			response := httptest.NewRecorder()
			got, ok := (&Server{}).requireIdempotencyKey(response, request)
			if ok != test.wantOK {
				t.Fatalf("requireIdempotencyKey() ok = %t, want %t", ok, test.wantOK)
			}
			if test.wantOK && got != test.value {
				t.Fatalf("requireIdempotencyKey() = %q, want %q", got, test.value)
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestBoundedMultipartFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fieldName string
		content   string
		maxBytes  int64
		want      string
		wantErr   bool
	}{
		{name: "one bounded file", fieldName: "file", content: "code\nTXQ-ABC\n", maxBytes: 64, want: "code\nTXQ-ABC\n"},
		{name: "wrong field", fieldName: "upload", content: "code\nTXQ-ABC\n", maxBytes: 64, wantErr: true},
		{name: "file too large", fieldName: "file", content: strings.Repeat("x", 65), maxBytes: 64, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile(test.fieldName, "results.csv")
			if err != nil {
				t.Fatalf("CreateFormFile(): %v", err)
			}
			if _, err := part.Write([]byte(test.content)); err != nil {
				t.Fatalf("write multipart file: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("close multipart writer: %v", err)
			}
			request := httptest.NewRequest(http.MethodPost, "/", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			got, err := boundedMultipartFile(response, request, "file", test.maxBytes)
			if test.wantErr && err == nil {
				t.Fatal("boundedMultipartFile() unexpectedly succeeded")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("boundedMultipartFile(): %v", err)
			}
			if string(got) != test.want {
				t.Fatalf("boundedMultipartFile() = %q, want %q", string(got), test.want)
			}
		})
	}
}

func TestActivityDescriptionResponseMapping(t *testing.T) {
	t.Parallel()

	game := mapActivityGame(activity.Game{GameInput: activity.GameInput{ID: "game-1", Name: "Coin", Description: "Game terms"}})
	if game.Description != "Game terms" {
		t.Fatalf("game description = %q", game.Description)
	}
	draw := mapLuckyDrawAdmin(activity.LuckyDraw{LuckyDrawInput: activity.LuckyDrawInput{ID: "draw-1", Name: "Draw", Description: "Draw terms"}})
	if draw.Description != "Draw terms" {
		t.Fatalf("draw description = %q", draw.Description)
	}
}

func TestCouponUpdateDistinguishesOmittedFieldsFromNull(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "coupon-nullable.sqlite"))
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := coupons.NewService(database.NewStore(db), func() time.Time {
		return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	})
	capMinor := int64(500)
	globalLimit := int64(100)
	userLimit := int64(2)
	expiresAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	created, err := service.Save(ctx, coupons.CouponInput{Code: "CLEAR100", Name: "Clear fields", Kind: coupons.KindPurchaseRecurring,
		DiscountMode: coupons.DiscountPercent, ValueMinorOrBPS: 1_000, PercentCapMinor: &capMinor, ExpiresAt: &expiresAt,
		GlobalUseLimit: &globalLimit, PerUserUseLimit: &userLimit, Active: true})
	if err != nil {
		t.Fatalf("Save(create): %v", err)
	}
	server := &Server{deps: Dependencies{Coupons: service}}

	var omitted couponRequest
	if err := json.Unmarshal([]byte(`{"name":"Preserved"}`), &omitted); err != nil {
		t.Fatalf("decode omitted request: %v", err)
	}
	preserved, err := server.couponInput(ctx, created.ID, omitted)
	if err != nil {
		t.Fatalf("couponInput(omitted): %v", err)
	}
	if preserved.PercentCapMinor == nil || preserved.ExpiresAt == nil || preserved.GlobalUseLimit == nil || preserved.PerUserUseLimit == nil {
		t.Fatalf("omitted nullable fields were cleared: %+v", preserved)
	}

	var cleared couponRequest
	if err := json.Unmarshal([]byte(`{"percentCapMinor":null,"expiresAt":null,"globalUseLimit":null,"perUserUseLimit":null}`), &cleared); err != nil {
		t.Fatalf("decode null request: %v", err)
	}
	input, err := server.couponInput(ctx, created.ID, cleared)
	if err != nil {
		t.Fatalf("couponInput(null): %v", err)
	}
	if input.PercentCapMinor != nil || input.ExpiresAt != nil || input.GlobalUseLimit != nil || input.PerUserUseLimit != nil {
		t.Fatalf("explicit null did not clear nullable fields: %+v", input)
	}
	updated, err := service.Save(ctx, input)
	if err != nil {
		t.Fatalf("Save(clear): %v", err)
	}
	if updated.PercentCapMinor != nil || updated.ExpiresAt != nil || updated.GlobalUseLimit != nil || updated.PerUserUseLimit != nil {
		t.Fatalf("cleared fields were not persisted: %+v", updated)
	}
}

func TestQuestionnaireClosesAtRequestAndResponseMapping(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "questionnaire-close.sqlite"))
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := questionnaires.NewService(database.NewStore(db), nil, func() time.Time { return now })
	closesAt := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	created, err := service.Save(ctx, questionnaires.QuestionnaireInput{Title: "Timed", FormURL: "https://forms.gle/abcdefgh",
		Status: questionnaires.StatusDraft, ClosesAt: &closesAt})
	if err != nil {
		t.Fatalf("Save(create): %v", err)
	}
	server := &Server{deps: Dependencies{Questionnaires: service}}

	var omitted questionnaireRequest
	if err := json.Unmarshal([]byte(`{"title":"Keep close"}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.ClosesAt.Set {
		t.Fatal("omitted closesAt was marked as set")
	}
	preserved, err := server.questionnaireInput(ctx, created.ID, omitted)
	if err != nil || preserved.ClosesAt == nil || !preserved.ClosesAt.Equal(closesAt) {
		t.Fatalf("omitted closesAt merge = (%+v, %v)", preserved.ClosesAt, err)
	}
	var cleared questionnaireRequest
	if err := json.Unmarshal([]byte(`{"closesAt":null}`), &cleared); err != nil {
		t.Fatal(err)
	}
	if !cleared.ClosesAt.Set || cleared.ClosesAt.Value != nil {
		t.Fatalf("null closesAt = %+v", cleared.ClosesAt)
	}
	clearedInput, err := server.questionnaireInput(ctx, created.ID, cleared)
	if err != nil || clearedInput.ClosesAt != nil {
		t.Fatalf("cleared closesAt merge = (%+v, %v)", clearedInput.ClosesAt, err)
	}
	response := mapQuestionnaire(questionnaires.Questionnaire{
		QuestionnaireInput: questionnaires.QuestionnaireInput{ID: "questionnaire-1", ClosesAt: &closesAt},
		ParticipantCount:   7,
		RewardedCount:      3,
	})
	if response.ClosesAt == nil || !response.ClosesAt.Equal(closesAt) {
		t.Fatalf("response closesAt = %v", response.ClosesAt)
	}
	if response.ParticipantCount != 7 || response.RewardedCount != 3 {
		t.Fatalf("response counters = (%d, %d), want (7, 3)", response.ParticipantCount, response.RewardedCount)
	}
}

func TestDecodeEmbyPreferencesRequiresDisabledLibraryIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		withPassword bool
		wantPassword string
		wantErr      bool
	}{
		{name: "setup requires libraries", body: `{"password":"secret"}`, withPassword: true, wantErr: true},
		{name: "preferences require libraries", body: `{"maxParentalRating":12}`, wantErr: true},
		{name: "null libraries are rejected", body: `{"disabledLibraryIds":null}`, wantErr: true},
		{name: "preferences reject setup password", body: `{"password":"secret","disabledLibraryIds":[]}`, wantErr: true},
		{name: "empty preferences list is present", body: `{"maxParentalRating":12,"disabledLibraryIds":[]}`},
		{name: "setup accepts a present list", body: `{"password":"secret","maxParentalRating":12,"disabledLibraryIds":["library-1"]}`, withPassword: true, wantPassword: "secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			preferences, password, err := decodeEmbyPreferences(response, request, test.withPassword)
			if test.wantErr && err == nil {
				t.Fatal("decodeEmbyPreferences() unexpectedly succeeded")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("decodeEmbyPreferences(): %v", err)
			}
			if !test.wantErr && preferences.DisabledLibraryIDs == nil {
				t.Fatal("disabledLibraryIds lost presence")
			}
			if password != test.wantPassword {
				t.Fatalf("password = %q, want %q", password, test.wantPassword)
			}
		})
	}
}
