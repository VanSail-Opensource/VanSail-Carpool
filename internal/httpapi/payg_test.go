package httpapi

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestPAYGReadIgnoresForeignOwnerQuery(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "payg.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := database.NewStore(db)
	first, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 80501}, false)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 80502}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id IN (?,?)`, first.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSquadProduct(ctx, database.SquadProductInput{GroupID: "owner-group", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveXNodeGroup(ctx, xnode.Group{ID: "owner-group", Name: "Owner group", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Minute).Add(-time.Minute).Format(time.RFC3339Nano)
	for index, user := range []model.User{first, second} {
		if _, err = db.ExecContext(ctx, `INSERT INTO xnode_payg_minutes(user_id,group_id,minute,traffic_bytes,spent_minor) VALUES(?,?,?,?,?)`, user.ID, "owner-group", at, (index+1)*101, (index+1)*3); err != nil {
			t.Fatal(err)
		}
	}
	server := Server{deps: Dependencies{XNode: xnode.NewService(s, nil, nil, nil, nil)}}
	request := httptest.NewRequest("GET", "/api/v1/payg?userId="+second.ID, nil)
	request = request.WithContext(context.WithValue(ctx, userContextKey, first))
	response := httptest.NewRecorder()
	server.ownedPAYG(response, request)
	var value xnode.PAYGView
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &value) != nil || value.TrafficBytes != "101" || value.Spent.Minor != "3" {
		t.Fatal("foreign-owner data exposed", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("financial view cacheable")
	}
}
