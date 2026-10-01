package whybroken_test

import (
	"testing"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/whybroken"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/event"
	"github.com/BaimPriyatna/repro/src/core/relation"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
	"github.com/BaimPriyatna/repro/src/graph/store"
)

// Graph layout used across most tests:
//
// web-app --depends_on--> auth-service --depends_on--> shared-lib
// isolated-tool (no edges to the above).
func setupGraph(t *testing.T) *repairmap.RepairMap {
	t.Helper()
	s := store.NewMemStore()

	entities := []*entity.Entity{
		{ID: "shared-lib", Kind: entity.KindPackage, Name: "shared-lib"},
		{ID: "auth-service", Kind: entity.KindService, Name: "auth-service"},
		{ID: "web-app", Kind: entity.KindModule, Name: "web-app"},
		{ID: "isolated-tool", Kind: entity.KindTool, Name: "isolated-tool"},
	}
	for _, e := range entities {
		if err := s.AddEntity(e); err != nil {
			t.Fatalf("AddEntity(%s): %v", e.ID, err)
		}
	}

	relations := []*relation.Relation{
		{ID: "r1", Kind: relation.KindDependsOn, FromID: "auth-service", ToID: "shared-lib"},
		{ID: "r2", Kind: relation.KindDependsOn, FromID: "web-app", ToID: "auth-service"},
	}
	for _, r := range relations {
		if err := s.AddRelation(r); err != nil {
			t.Fatalf("AddRelation(%s): %v", r.ID, err)
		}
	}

	rm, err := repairmap.New(s)
	if err != nil {
		t.Fatalf("repairmap.New: %v", err)
	}
	return rm
}

func makeSnap(id snapshot.ID, ts time.Time, data map[string]any) *snapshot.Snapshot {
	return &snapshot.Snapshot{
		ID:            id,
		Timestamp:     ts,
		SchemaVersion: snapshot.SchemaVersion,
		Data:          data,
		Source:        snapshot.SourceManual,
	}
}

func mustEvent(t *testing.T, typ event.Type, subject string, opts ...event.Option) *event.Event {
	t.Helper()
	evt, err := event.New(typ, event.SourceWatchdog, subject, opts...)
	if err != nil {
		t.Fatalf("event.New: %v", err)
	}
	return evt
}

func findingByType(result *coreanalysis.AnalysisResult, typ string) []coreanalysis.Finding {
	out := make([]coreanalysis.Finding, 0)
	for _, f := range result.Findings {
		if f.Type == typ {
			out = append(out, f)
		}
	}
	return out
}

func TestNew_Validation(t *testing.T) {
	rm := setupGraph(t)
	before := makeSnap("s-before", time.Now(), map[string]any{})
	after := makeSnap("s-after", time.Now(), map[string]any{})

	if _, err := whybroken.New(nil, before, after, nil); err == nil {
		t.Error("expected error for nil repair map")
	}
	if _, err := whybroken.New(rm, nil, after, nil); err == nil {
		t.Error("expected error for nil before snapshot")
	}
	if _, err := whybroken.New(rm, before, nil, nil); err == nil {
		t.Error("expected error for nil after snapshot")
	}

	wb, err := whybroken.New(rm, before, after, nil)
	if err != nil {
		t.Fatalf("New with valid inputs: %v", err)
	}
	if wb.Name() != whybroken.AnalyzerName {
		t.Errorf("Name() = %q, want %q", wb.Name(), whybroken.AnalyzerName)
	}
	if wb.Version() == "" {
		t.Error("Version() should not be empty")
	}
}

func TestAnalyzer_Interface(t *testing.T) {
	rm := setupGraph(t)
	before := makeSnap("s-a", time.Now(), map[string]any{})
	after := makeSnap("s-b", time.Now(), map[string]any{})

	wb, err := whybroken.New(rm, before, after, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var _ analysis.Analyzer = wb
}

func TestAnalyze_EmptyDiffAndEvents_StatusOK(t *testing.T) {
	rm := setupGraph(t)
	data := map[string]any{"runtime": map[string]any{"go": "1.22"}}
	before := makeSnap("s-same-a", time.Now(), data)
	after := makeSnap("s-same-b", time.Now(), data)

	wb, err := whybroken.New(rm, before, after, nil, whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Status != coreanalysis.StatusOK {
		t.Errorf("Status = %q, want %q", result.Status, coreanalysis.StatusOK)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	if len(result.RelatedSnapshots) != 2 {
		t.Errorf("RelatedSnapshots = %d, want 2", len(result.RelatedSnapshots))
	}
}

func TestAnalyze_ConfirmedChange_DiffEventAndGraph(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	before := makeSnap("s-conf-a", t0, map[string]any{
		"packages": map[string]any{
			"shared-lib": map[string]any{"version": "1.0.0"},
		},
	})
	after := makeSnap("s-conf-b", t0.Add(time.Hour), map[string]any{
		"packages": map[string]any{
			"shared-lib": map[string]any{"version": "2.0.0"},
		},
	})

	evt := mustEvent(t, event.TypePackageInstalled, "shared-lib",
		event.WithID("evt-shared-lib"),
		event.WithTimestamp(t0.Add(30*time.Minute)),
	)

	wb, err := whybroken.New(rm, before, after, []*event.Event{evt}, whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Status != coreanalysis.StatusFindings {
		t.Fatalf("Status = %q, want findings", result.Status)
	}

	confirmed := findingByType(result, whybroken.FindingTypeConfirmedChange)
	if len(confirmed) == 0 {
		t.Fatalf("expected at least one confirmed_change, got findings: %+v", result.Findings)
	}
	f := confirmed[0]
	if f.Subject != "shared-lib" {
		t.Errorf("Subject = %q, want shared-lib", f.Subject)
	}
	if f.Confidence != coreanalysis.ConfidenceConfirmed {
		t.Errorf("Confidence = %q, want confirmed", f.Confidence)
	}
	if !f.HasEvidence() {
		t.Error("confirmed finding must have evidence")
	}

	hasDiff := false
	hasEvent := false
	hasRelation := false
	for _, e := range f.Evidence {
		switch e.Kind {
		case coreanalysis.EvidenceKindDiff:
			hasDiff = true
		case coreanalysis.EvidenceKindEvent:
			hasEvent = true
		case coreanalysis.EvidenceKindRelation:
			hasRelation = true
		}
	}
	if !hasDiff || !hasEvent || !hasRelation {
		t.Errorf("evidence incomplete: diff=%v event=%v relation=%v", hasDiff, hasEvent, hasRelation)
	}
}

func TestAnalyze_LikelyContributor_DiffDirectNoEvent(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 11, 0, 0, 0, time.UTC)

	before := makeSnap("s-likely-a", t0, map[string]any{
		"packages": map[string]any{
			"auth-service": map[string]any{"version": "1.0"},
		},
	})
	after := makeSnap("s-likely-b", t0.Add(time.Hour), map[string]any{
		"packages": map[string]any{
			"auth-service": map[string]any{"version": "1.1"},
		},
	})

	wb, err := whybroken.New(rm, before, after, nil, whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	likely := findingByType(result, whybroken.FindingTypeLikelyContributor)
	if len(likely) == 0 {
		t.Fatalf("expected likely_contributor, got: %+v", result.Findings)
	}
	if likely[0].Subject != "auth-service" {
		t.Errorf("Subject = %q, want auth-service", likely[0].Subject)
	}
	if likely[0].Confidence != coreanalysis.ConfidenceHigh {
		t.Errorf("Confidence = %q, want high", likely[0].Confidence)
	}
	if !likely[0].HasEvidence() {
		t.Error("likely finding must have evidence")
	}
}

func TestAnalyze_PossibleContributor_TransitiveDiff(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	// shared-lib is transitive from web-app (web-app → auth-service → shared-lib)
	before := makeSnap("s-poss-a", t0, map[string]any{
		"packages": map[string]any{
			"shared-lib": map[string]any{"version": "1.0"},
		},
	})
	after := makeSnap("s-poss-b", t0.Add(time.Hour), map[string]any{
		"packages": map[string]any{
			"shared-lib": map[string]any{"version": "1.0.1"},
		},
	})

	wb, err := whybroken.New(rm, before, after, nil, whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	possible := findingByType(result, whybroken.FindingTypePossibleContributor)
	if len(possible) == 0 {
		t.Fatalf("expected possible_contributor for transitive, got: %+v", result.Findings)
	}
	if possible[0].Subject != "shared-lib" {
		t.Errorf("Subject = %q, want shared-lib", possible[0].Subject)
	}
}

func TestAnalyze_PossibleContributor_EventGraphNoDiff(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)

	// Identical snapshots → no Diff
	data := map[string]any{"packages": map[string]any{"auth-service": "1.0"}}
	before := makeSnap("s-ev-a", t0, data)
	after := makeSnap("s-ev-b", t0.Add(time.Hour), data)

	evt := mustEvent(t, event.TypeConfigChanged, "auth-service",
		event.WithID("evt-auth-only"),
		event.WithTimestamp(t0.Add(20*time.Minute)),
	)

	wb, err := whybroken.New(rm, before, after, []*event.Event{evt}, whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Event + graph link, no Diff → possible (never likely/confirmed)
	possible := findingByType(result, whybroken.FindingTypePossibleContributor)
	if len(possible) == 0 {
		t.Fatalf("expected possible_contributor, got: %+v", result.Findings)
	}
	for _, f := range result.Findings {
		if f.Type == whybroken.FindingTypeConfirmedChange || f.Type == whybroken.FindingTypeLikelyContributor {
			t.Errorf("event-only must not yield %s", f.Type)
		}
	}
}

func TestAnalyze_Unknown_OrphanEventTimeAlone(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 14, 0, 0, 0, time.UTC)

	data := map[string]any{"runtime": map[string]any{"go": "1.22"}}
	before := makeSnap("s-unk-a", t0, data)
	after := makeSnap("s-unk-b", t0.Add(time.Hour), data)

	// Event subject does not match any graph entity
	evt := mustEvent(t, event.TypeCommandExecuted, "curl https://example.com",
		event.WithID("evt-orphan"),
		event.WithTimestamp(t0.Add(15*time.Minute)),
	)

	wb, err := whybroken.New(rm, before, after, []*event.Event{evt}, whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	unknown := findingByType(result, whybroken.FindingTypeUnknown)
	if len(unknown) != 1 {
		t.Fatalf("expected 1 unknown finding, got %d: %+v", len(unknown), result.Findings)
	}
	if unknown[0].Confidence != coreanalysis.ConfidenceUnknown {
		t.Errorf("Confidence = %q, want unknown", unknown[0].Confidence)
	}
	if !unknown[0].HasEvidence() {
		t.Error("unknown finding must still carry event evidence")
	}
	for _, f := range result.Findings {
		if f.Type == whybroken.FindingTypeConfirmedChange || f.Type == whybroken.FindingTypeLikelyContributor {
			t.Errorf("time-proximity alone must not yield %s", f.Type)
		}
	}
}

func TestAnalyze_Determinism(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)

	before := makeSnap("s-det-a", t0, map[string]any{
		"packages": map[string]any{
			"shared-lib":   map[string]any{"version": "1.0"},
			"auth-service": map[string]any{"version": "1.0"},
		},
	})
	after := makeSnap("s-det-b", t0.Add(time.Hour), map[string]any{
		"packages": map[string]any{
			"shared-lib":   map[string]any{"version": "2.0"},
			"auth-service": map[string]any{"version": "1.1"},
		},
	})

	evt := mustEvent(t, event.TypePackageInstalled, "shared-lib",
		event.WithID("evt-det"),
		event.WithTimestamp(t0.Add(30*time.Minute)),
	)

	run := func() *coreanalysis.AnalysisResult {
		wb, err := whybroken.New(rm, before, after, []*event.Event{evt}, whybroken.WithSubject("web-app"))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		result, err := wb.Analyze()
		if err != nil {
			t.Fatalf("Analyze: %v", err)
		}
		return result
	}

	a := run()
	b := run()

	if len(a.Findings) != len(b.Findings) {
		t.Fatalf("finding count mismatch: %d vs %d", len(a.Findings), len(b.Findings))
	}
	for i := range a.Findings {
		if a.Findings[i].ID != b.Findings[i].ID {
			t.Errorf("finding[%d].ID = %q vs %q", i, a.Findings[i].ID, b.Findings[i].ID)
		}
		if a.Findings[i].Type != b.Findings[i].Type {
			t.Errorf("finding[%d].Type = %q vs %q", i, a.Findings[i].Type, b.Findings[i].Type)
		}
		if a.Findings[i].Subject != b.Findings[i].Subject {
			t.Errorf("finding[%d].Subject = %q vs %q", i, a.Findings[i].Subject, b.Findings[i].Subject)
		}
	}
}

func TestAnalyze_Sorting_ConfirmedBeforeUnknown(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)

	before := makeSnap("s-sort-a", t0, map[string]any{
		"packages": map[string]any{"shared-lib": map[string]any{"version": "1.0"}},
	})
	after := makeSnap("s-sort-b", t0.Add(time.Hour), map[string]any{
		"packages": map[string]any{"shared-lib": map[string]any{"version": "2.0"}},
	})

	evtConfirmed := mustEvent(t, event.TypePackageInstalled, "shared-lib",
		event.WithID("evt-sort-conf"),
		event.WithTimestamp(t0.Add(10*time.Minute)),
	)
	evtOrphan := mustEvent(t, event.TypeCommandExecuted, "unrelated-cmd",
		event.WithID("evt-sort-orphan"),
		event.WithTimestamp(t0.Add(5*time.Minute)),
	)

	wb, err := whybroken.New(rm, before, after, []*event.Event{evtOrphan, evtConfirmed},
		whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Findings) < 2 {
		t.Fatalf("expected >=2 findings, got %d", len(result.Findings))
	}
	if result.Findings[0].Type != whybroken.FindingTypeConfirmedChange {
		t.Errorf("first finding type = %q, want confirmed_change", result.Findings[0].Type)
	}
	last := result.Findings[len(result.Findings)-1]
	if last.Type != whybroken.FindingTypeUnknown {
		t.Errorf("last finding type = %q, want unknown", last.Type)
	}
}

func TestAnalyze_EveryFindingHasEvidence(t *testing.T) {
	rm := setupGraph(t)
	t0 := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)

	before := makeSnap("s-evd-a", t0, map[string]any{
		"packages": map[string]any{
			"auth-service": map[string]any{"version": "1.0"},
			"shared-lib":   map[string]any{"version": "1.0"},
		},
	})
	after := makeSnap("s-evd-b", t0.Add(time.Hour), map[string]any{
		"packages": map[string]any{
			"auth-service": map[string]any{"version": "1.1"},
			"shared-lib":   map[string]any{"version": "2.0"},
		},
	})

	events := []*event.Event{
		mustEvent(t, event.TypePackageInstalled, "shared-lib", event.WithID("evt-e1"), event.WithTimestamp(t0.Add(time.Minute))),
		mustEvent(t, event.TypeCommandExecuted, "noise", event.WithID("evt-e2"), event.WithTimestamp(t0.Add(2*time.Minute))),
	}

	wb, err := whybroken.New(rm, before, after, events, whybroken.WithSubject("web-app"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := wb.Analyze()
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	for _, f := range result.Findings {
		if !f.HasEvidence() {
			t.Errorf("finding %s (%s) has no evidence (missing evidence)", f.ID, f.Type)
		}
	}
}
