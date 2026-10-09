package scanner

import "testing"

func TestRequiresReviewPredicate(t *testing.T) {
	tests := []struct {
		name      string
		candidate Candidate
		want      bool
	}{
		{name: "weak_single_signal_is_explanatory_only", candidate: Candidate{Confidence: ConfidenceWeak, ReviewReasons: []string{"single_structural_signal"}}, want: false},
		{name: "weak_generic_suspicion", candidate: Candidate{Confidence: ConfidenceWeak, ReviewReasons: []string{"generic_name_review"}}, want: true},
		{name: "weak_repeated_layer_suspicion", candidate: Candidate{Confidence: ConfidenceWeak, ReviewReasons: []string{"repeated_layer_pattern"}}, want: true},
		{name: "strong_without_suspicion", candidate: Candidate{Confidence: ConfidenceStrong}, want: false},
		{name: "supported_without_suspicion", candidate: Candidate{Confidence: ConfidenceSupported}, want: false},
		{name: "supported_explicit_suspicion", candidate: Candidate{Confidence: ConfidenceSupported, ReviewReasons: []string{"generic_name_review"}}, want: true},
		{name: "conflicted_even_without_reason", candidate: Candidate{Confidence: ConfidenceConflicted}, want: true},
		{name: "unresolved_even_without_reason", candidate: Candidate{Confidence: ConfidenceUnresolved}, want: true},
		{name: "unknown_future_reason_fails_closed", candidate: Candidate{Confidence: ConfidenceStrong, ReviewReasons: []string{"future_review_reason"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requiresReview(tt.candidate); got != tt.want {
				t.Fatalf("requiresReview(%#v)=%v, want %v", tt.candidate, got, tt.want)
			}
		})
	}
}

func TestReviewSemanticsChangeDigestWithoutRenamingCandidate(t *testing.T) {
	initial := remediationSnapshot(t, "review-digest", "pkg", []string{"pkg/a.go"}, map[string]string{"package": "pkg"})
	initialSet, err := Derive(initial)
	if err != nil {
		t.Fatal(err)
	}
	initialCandidate, ok := candidateByCoordinate(initialSet, "pkg")
	if !ok {
		t.Fatal("initial candidate missing")
	}
	if initialCandidate.Confidence != ConfidenceStrong || initialCandidate.ReviewRequired {
		t.Fatalf("initial candidate=%#v", initialCandidate)
	}

	conflict := ObservationBatch{Producer: initial.Producer, Subject: initial.Subject, Nodes: append([]Node(nil), initial.Nodes...), Edges: append([]Edge(nil), initial.Edges...), Evidence: append([]Evidence(nil), initial.Evidence...), Coverage: append([]Coverage(nil), initial.Coverage...), Unknowns: append([]Unknown(nil), initial.Unknowns...), Diagnostics: append([]Diagnostic(nil), initial.Diagnostics...)}
	other := Node{ID: NodeID(initial.Subject.ID, "go.package", "pkg#package/other"), Kind: "go.package", Coordinate: "pkg#package/other", Name: "other", Attributes: map[string]string{"directory": "pkg", "package": "other"}}
	conflict.Nodes = append(conflict.Nodes, other)
	conflictSnapshot, err := BuildSnapshot(conflict, initial.Config)
	if err != nil {
		t.Fatal(err)
	}
	conflictSet, err := Derive(conflictSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	conflictCandidate, ok := candidateByCoordinate(conflictSet, "pkg")
	if !ok {
		t.Fatal("conflicted candidate missing")
	}
	if conflictCandidate.ID != initialCandidate.ID {
		t.Fatalf("review change renamed candidate: %s != %s", conflictCandidate.ID, initialCandidate.ID)
	}
	if conflictCandidate.Confidence != ConfidenceConflicted || !conflictCandidate.ReviewRequired {
		t.Fatalf("conflicted candidate=%#v", conflictCandidate)
	}
	if conflictSet.Digest == initialSet.Digest {
		t.Fatal("review/confidence change did not change logical candidate digest")
	}
}
