package main

import "testing"

func TestParseBLSReleasePage(t *testing.T) {
	data := []byte(`<table><tr><td>Friday, September 4, 2026</td><td>8:30 AM</td><td>Employment Situation</td></tr><tr><td>October 2, 2026</td><td>Employment Situation</td></tr></table>`)
	events, err := parseBLSReleasePage(data, "NFP", "非农就业报告", "employment", blsEmploymentURL, "desc", "Employment Situation")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].ID != "macro-official-nfp-20260904" || events[0].StartsAt != "2026-09-04T20:30:00+08:00" {
		t.Fatalf("unexpected BLS events: %+v", events)
	}
}

func TestParseFOMCPage(t *testing.T) {
	data := []byte(`<h3>2026</h3><div class="fomc-meeting"><div>September</div><div>16-17</div></div>`)
	events, err := parseFOMCPage(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "macro-official-fomc-20260916" || events[0].StartsAt != "2026-09-18T02:00:00+08:00" {
		t.Fatalf("unexpected FOMC events: %+v", events)
	}
}

func TestParseBEASchedulePage(t *testing.T) {
	data := []byte(`<table><tr><td>September 30, 2026</td><td>8:30 AM</td><td>Personal Income and Outlays, August 2026</td></tr></table>`)
	events, err := parseBEASchedulePage(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "macro-official-pce-20260930" || events[0].StartsAt != "2026-09-30T20:30:00+08:00" {
		t.Fatalf("unexpected BEA events: %+v", events)
	}
}

func TestContainsMacroEventOnDateKeepsMissingReferenceDate(t *testing.T) {
	reference := MacroEvent{Code: "NFP", StartsAt: "2026-09-04T20:30:00+08:00"}
	if containsMacroEventOnDate([]MacroEvent{{Code: "NFP", StartsAt: "2026-02-05T21:30:00+08:00"}}, reference) {
		t.Fatal("NFP reference should remain when the official page only returns another date")
	}
	if !containsMacroEventOnDate([]MacroEvent{{Code: "nfp", StartsAt: "2026-09-04T20:30:00+08:00"}}, reference) {
		t.Fatal("matching NFP date should remove the reference event")
	}
}
