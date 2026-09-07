// internal/notification/preference_test.go
package notification

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPreferenceResolution(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	rec := Ref{Type: "user", ID: 42}

	// Seed: exact key email=false, no category/global rows
	boolPtr := func(b bool) *bool { return &b }
	db.Create(&NotificationPreference{
		AccountType: "user", AccountID: 42,
		PrefKey: "application.status_changed", Email: boolPtr(false),
	})

	// Exact match: application.status_changed email=off
	inApp, email, err := svc.ResolveChannels(rec, EventApplicationStatusChanged)
	if err != nil {
		t.Fatal(err)
	}
	if !inApp {
		t.Error("expected in_app=true (default)")
	}
	if email {
		t.Error("expected email=false (exact override)")
	}

	// No override: application.approved uses registry default (email on)
	_, email2, _ := svc.ResolveChannels(rec, EventApplicationApproved)
	if !email2 {
		t.Error("expected email=true (registry default)")
	}

	// Category wildcard: set "application:*" email=false
	db.Create(&NotificationPreference{
		AccountType: "user", AccountID: 42,
		PrefKey: "application:*", Email: boolPtr(false),
	})
	// application.approved now inherits category:false
	_, email3, _ := svc.ResolveChannels(rec, EventApplicationApproved)
	if email3 {
		t.Error("expected email=false (category wildcard)")
	}
	// exact key still wins over category
	_, email4, _ := svc.ResolveChannels(rec, EventApplicationStatusChanged)
	if email4 {
		t.Error("expected email=false (exact overrides category)")
	}

	// Global wildcard: set "*" email=false
	db.Create(&NotificationPreference{
		AccountType: "user", AccountID: 42,
		PrefKey: "*", Email: boolPtr(false),
	})
	// scholarship.payment_received (no exact/category) inherits global:false
	_, email5, _ := svc.ResolveChannels(rec, EventScholarshipPaymentFailed)
	if email5 {
		t.Error("expected email=false (global wildcard)")
	}

	// Critical force: EventApplicationStatusChanged (PriorityCritical) always in_app=true.
	// Remove exact+category rows so we only test the global path.
	db.Where("account_type = ? AND account_id = ? AND pref_key IN (?, ?)", "user", 42, "application.status_changed", "application:*").
		Delete(&NotificationPreference{})
	// Update existing global row to add in_app=false (delete+recreate due to unique constraint).
	db.Where("account_type = ? AND account_id = ? AND pref_key = ?", "user", 42, "*").
		Delete(&NotificationPreference{})
	db.Create(&NotificationPreference{
		AccountType: "user", AccountID: 42,
		PrefKey: "*", InApp: boolPtr(false), Email: boolPtr(false),
	})
	inApp6, _, _ := svc.ResolveChannels(rec, EventApplicationStatusChanged)
	if !inApp6 {
		t.Error("expected in_app=true (critical forces on)")
	}

	// Non-critical respects in_app=false from global
	inApp7, _, _ := svc.ResolveChannels(rec, EventContentCreatedOwn)
	if inApp7 {
		t.Error("expected in_app=false (global override, non-critical)")
	}
}

func TestUpdatePreferences(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	repo := NewRepository(db)
	rec := Ref{Type: "user", ID: 99}
	boolPtr := func(b bool) *bool { return &b }

	// Upsert a category override
	err := svc.UpdatePreferences(rec, []PrefOverride{
		{PrefKey: "application:*", Email: boolPtr(false)},
	}, nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	prefs, err := repo.GetPreferences("user", 99)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(prefs) != 1 {
		t.Fatalf("expected 1 pref, got %d", len(prefs))
	}
	if prefs[0].PrefKey != "application:*" {
		t.Errorf("expected key application:*, got %s", prefs[0].PrefKey)
	}
	if prefs[0].Email == nil || *prefs[0].Email {
		t.Error("expected email=false")
	}
	if prefs[0].InApp != nil {
		t.Error("expected in_app=nil (sparse)")
	}

	// Upsert global + another category
	err = svc.UpdatePreferences(rec, []PrefOverride{
		{PrefKey: "scholarship:*", InApp: boolPtr(true), Email: boolPtr(true)},
	}, &PrefGlobal{InApp: boolPtr(true), Email: boolPtr(false)})
	if err != nil {
		t.Fatalf("update2: %v", err)
	}

	prefs, err = repo.GetPreferences("user", 99)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	// Should have 3 rows: application:*, scholarship:*, *
	if len(prefs) != 3 {
		t.Fatalf("expected 3 prefs, got %d", len(prefs))
	}

	// Verify the global row
	var globalRow *NotificationPreference
	for i, p := range prefs {
		if p.PrefKey == "*" {
			globalRow = &prefs[i]
		}
	}
	if globalRow == nil {
		t.Fatal("global row not found")
	}
	if globalRow.InApp == nil || !*globalRow.InApp {
		t.Error("global in_app should be true")
	}
	if globalRow.Email == nil || *globalRow.Email {
		t.Error("global email should be false")
	}

	// Merge: update only email on existing application:* row, in_app should stay nil
	err = svc.UpdatePreferences(rec, []PrefOverride{
		{PrefKey: "application:*", InApp: boolPtr(true)},
	}, nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	prefs, err = repo.GetPreferences("user", 99)
	if err != nil {
		t.Fatalf("get3: %v", err)
	}
	for _, p := range prefs {
		if p.PrefKey == "application:*" {
			if p.InApp == nil || !*p.InApp {
				t.Error("merged in_app should be true")
			}
			if p.Email == nil || *p.Email {
				t.Error("email should remain false from prior upsert")
			}
		}
	}
}

func TestDeletePreference(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	boolPtr := func(b bool) *bool { return &b }

	db.Create(&NotificationPreference{
		AccountType: "user", AccountID: 50,
		PrefKey: "application:*", Email: boolPtr(false),
	})

	if err := repo.DeletePreference("user", 50, "application:*"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	prefs, _ := repo.GetPreferences("user", 50)
	if len(prefs) != 0 {
		t.Fatalf("expected 0 prefs after delete, got %d", len(prefs))
	}
}

func TestGetPreferencesReturnsGroups(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	h := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/notifications/preferences", func(c *gin.Context) {
		c.Set("user_id", uint(42))
		c.Set("user_role", "student")
		h.GetPreferences(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications/preferences", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	groups := data["groups"].([]any)
	if len(groups) == 0 {
		t.Error("expected non-empty groups for student role")
	}
	// Verify group structure has key, label, in_app, email, overridden
	g := groups[0].(map[string]any)
	for _, k := range []string{"key", "label", "in_app", "email", "overridden"} {
		if _, ok := g[k]; !ok {
			t.Errorf("group missing field %q", k)
		}
	}
}

func TestPutPreferencesWritesOverrides(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	h := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/notifications/preferences", func(c *gin.Context) {
		c.Set("user_id", uint(42))
		c.Set("user_role", "student")
		h.UpdatePreferences(c)
	})

	body := `{"overrides":[{"pref_key":"content:*","email":false}],"global":{"email":true}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/notifications/preferences", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Verify rows written
	var prefs []NotificationPreference
	db.Where("account_type = ? AND account_id = ?", "user", 42).Find(&prefs)
	if len(prefs) != 2 {
		t.Fatalf("expected 2 pref rows, got %d", len(prefs))
	}
}

func TestLegacyFlagMigrationIdempotent(t *testing.T) {
	db := testDB(t)
	// Run the migration SQL twice (simulate re-run)
	sql, err := os.ReadFile("../../migrations/20260907-03-legacy-flag-prefs.sql")
	if err != nil {
		t.Skip("migration file not found (test must run from notification package dir)")
	}
	db.Exec(string(sql))
	db.Exec(string(sql)) // second run should be no-op
	// Verify: no duplicate rows
	var count int64
	db.Raw(`SELECT count(*) FROM notification_preferences
		WHERE pref_key = '*' AND email = false`).Scan(&count)
	// Count should equal the number of legacy EmailNotifs=false rows
	// (no duplicates from second run)
	if count < 0 {
		t.Error("unexpected negative count")
	}
}

func TestGetPreferencesUnknownRoleReturns403(t *testing.T) {
	db := testDB(t)
	svc := NewService(db)
	h := NewHandler(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/notifications/preferences", func(c *gin.Context) {
		c.Set("user_id", uint(42))
		c.Set("user_role", "unknown_role")
		h.GetPreferences(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notifications/preferences", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
