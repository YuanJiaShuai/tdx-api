package workbench

import (
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

func newHistoricalBacktestTestStore(t *testing.T) *AppStore {
	t.Helper()
	db, err := sql.Open("sqlite", "file:historical-backtest-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	store := &AppStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestHistoricalBacktestRunReadsStoredCurrentDate(t *testing.T) {
	store := newHistoricalBacktestTestStore(t)
	run, err := store.CreateHistoricalBacktestRun(HistoricalBacktestRun{
		StartDate: "2026-06-01", EndDate: "2026-06-02", StrategyIDs: []string{"strategy-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateHistoricalBacktestProgress(run.ID, 2, 1, "2026-06-01", 4606, 15); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetHistoricalBacktestRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CurrentDate != "2026-06-01" {
		t.Fatalf("current date = %q, want stored replay date", loaded.CurrentDate)
	}
}
