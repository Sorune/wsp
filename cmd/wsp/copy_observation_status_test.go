package main

import "testing"

func TestCopyObservationStatus(t *testing.T) {
	cases := []struct {
		label     string
		decisions []string
		status    string
	}{
		{"no copies", nil, "OK"},
		{"aligned", []string{"ALIGNED", "RELATED", "TRACKED"}, "OK"},
		{"copy absent", []string{"UNAVAILABLE"}, "PARTIAL"},
		{"dirty parent", []string{"UNTRACKED_DIRTY_PARENT"}, "PARTIAL"},
		{"review", []string{"ALIGNED", "REVIEW_REQUIRED"}, "PARTIAL"},
		{"legacy placeholder", []string{"UNCONFIGURED_COPY"}, "PARTIAL"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			rows := make([]copyView, 0, len(tc.decisions))
			for _, decision := range tc.decisions {
				rows = append(rows, copyView{Decision: decision})
			}
			if got := copyObservationStatus(rows); got != tc.status {
				t.Fatalf("%v classified %s, want %s", tc.decisions, got, tc.status)
			}
		})
	}
}
