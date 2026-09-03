package workbench

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

func newMacroEventTestStore(t *testing.T) *AppStore {
	t.Helper()
	db, err := sql.Open("sqlite", "file:macro-events-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	store := &AppStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestMacroEventsDefaultsAndFilters(t *testing.T) {
	store := newMacroEventTestStore(t)
	if err := store.ensureMacroEvents(); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListMacroEvents("", "", "inflation", "high")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("expected four high-impact inflation events, got %d", len(items))
	}
	if items[0].Code != "CPI" || items[0].Acknowledged {
		t.Fatalf("unexpected first default event: %+v", items[0])
	}
}

func TestMacroEventValidationAndAcknowledgement(t *testing.T) {
	store := newMacroEventTestStore(t)
	if _, err := store.UpsertMacroEvent(MacroEvent{Code: "cpi", Name: "CPI", StartsAt: "2026-09-10 20:30"}); err == nil {
		t.Fatal("expected RFC3339 validation error")
	}
	event, err := store.UpsertMacroEvent(MacroEvent{Code: "cpi", Name: "CPI", StartsAt: "2026-09-10T20:30:00+08:00"})
	if err != nil {
		t.Fatal(err)
	}
	if event.Code != "CPI" || event.Country != "US" || event.Impact != "medium" {
		t.Fatalf("normalization failed: %+v", event)
	}
	if _, err := store.SetMacroEventAcknowledged(event.ID, true); err != nil {
		t.Fatal(err)
	}
	event.Description = "updated"
	updated, err := store.UpsertMacroEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Acknowledged {
		t.Fatal("editing an acknowledged event should preserve acknowledgement")
	}
	if err := store.DeleteMacroEvent("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestMacroAlertSettingsAndState(t *testing.T) {
	store := newMacroEventTestStore(t)
	if err := store.ensureMacroEvents(); err != nil {
		t.Fatal(err)
	}
	settings, err := store.GetMacroAlertSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.LeadMinutes != 240 || settings.WindowBeforeMinutes != 240 || settings.WindowAfterMinutes != 120 {
		t.Fatalf("unexpected default settings: %+v", settings)
	}
	event := MacroEvent{Impact: "high", StartsAt: "2026-09-10T20:30:00+08:00"}
	startsAt, _ := time.Parse(time.RFC3339, event.StartsAt)
	if got := MacroEventAlertState(event, settings, startsAt.Add(-5*time.Hour)); got != "scheduled" {
		t.Fatalf("expected scheduled, got %s", got)
	}
	if got := MacroEventAlertState(event, settings, startsAt.Add(-time.Hour)); got != "window_active" {
		t.Fatalf("expected active window, got %s", got)
	}
	if got := MacroEventAlertState(event, settings, startsAt.Add(3*time.Hour)); got != "released" {
		t.Fatalf("expected released, got %s", got)
	}
	settings.WindowBeforeMinutes = 999999
	updated, err := store.UpsertMacroAlertSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if updated.WindowBeforeMinutes != 7*24*60 {
		t.Fatalf("expected settings clamp, got %d", updated.WindowBeforeMinutes)
	}
}

func TestMacroAlertDeliveryIsIdempotent(t *testing.T) {
	store := newMacroEventTestStore(t)
	claimed, err := store.ClaimMacroAlertDelivery("event-1", "hook-1", "alert_due")
	if err != nil || !claimed {
		t.Fatalf("expected first delivery claim, claimed=%v err=%v", claimed, err)
	}
	claimed, err = store.ClaimMacroAlertDelivery("event-1", "hook-1", "alert_due")
	if err != nil || claimed {
		t.Fatalf("expected duplicate delivery to be ignored, claimed=%v err=%v", claimed, err)
	}
}
