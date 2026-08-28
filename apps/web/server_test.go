package main

import "testing"

func TestNormalizeMinuteDate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "compact", in: "20260610", want: "20260610"},
		{name: "hyphenated", in: "2026-06-10", want: "20260610"},
		{name: "trim spaces", in: " 2026-06-10 ", want: "20260610"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMinuteDate(tt.in)
			if err != nil {
				t.Fatalf("normalizeMinuteDate() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeMinuteDate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeMinuteDateRejectsInvalidInput(t *testing.T) {
	if _, err := normalizeMinuteDate("2026/06/10"); err == nil {
		t.Fatal("normalizeMinuteDate() expected error for invalid date")
	}
}
