package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

// ctx is the background context used across store tests.
var ctx = context.Background()

func openTemp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleOffer(key string, validTo time.Time) api.Offer {
	from, to := 20000.0, 30000.0
	return api.Offer{
		JobOfferKey:     key,
		Title:           "Go Developer",
		Company:         "Acme",
		Division:        "IT",
		ExperienceLevel: "Senior",
		Salary:          &api.Salary{From: &from, To: &to, Currency: "PLN"},
		IsRemote:        true,
		ValidTo:         validTo,
	}
}

func TestSeedDefaultProfile(t *testing.T) {
	s := openTemp(t)
	p, err := s.DefaultProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != DefaultProfileName || !p.IsDefault {
		t.Errorf("unexpected default profile: %+v", p)
	}
}

func TestOfferUpsertAndGet(t *testing.T) {
	s := openTemp(t)
	o := sampleOffer("k1", time.Now().Add(24*time.Hour))
	if err := s.UpsertOffers(ctx, []api.Offer{o}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetOffer(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Go Developer" || got.Salary == nil || *got.Salary.From != 20000 {
		t.Errorf("roundtrip mismatch: %+v", got)
	}

	// Upsert again with a changed title; should update, not duplicate.
	o.Title = "Senior Go Developer"
	if err := s.UpsertOffers(ctx, []api.Offer{o}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetOffer(ctx, "k1")
	if got.Title != "Senior Go Developer" {
		t.Errorf("upsert did not update title: %q", got.Title)
	}
}

func TestTrackerLifecycle(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)

	// Cannot track an uncached offer.
	if err := s.AddTracked(ctx, "ghost", pid); err == nil {
		t.Error("expected error tracking uncached offer")
	}

	s.UpsertOffers(ctx, []api.Offer{sampleOffer("k1", time.Now().Add(24*time.Hour))})
	if err := s.AddTracked(ctx, "k1", pid); err != nil {
		t.Fatal(err)
	}
	// Idempotent add.
	if err := s.AddTracked(ctx, "k1", pid); err != nil {
		t.Fatal(err)
	}

	if err := s.SetStatus(ctx, "k1", pid, StatusApplied); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStatus(ctx, "k1", pid, "bogus"); err == nil {
		t.Error("expected invalid status error")
	}

	items, err := s.ListTracked(ctx, pid, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != StatusApplied {
		t.Fatalf("unexpected tracked: %+v", items)
	}

	// Filter by status.
	none, _ := s.ListTracked(ctx, pid, StatusInterview)
	if len(none) != 0 {
		t.Errorf("expected no interview-stage items, got %d", len(none))
	}
}

func TestExpireStale(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)

	s.UpsertOffers(ctx, []api.Offer{sampleOffer("old", time.Now().Add(-time.Hour))})
	s.AddTracked(ctx, "old", pid)

	items, err := s.ListTracked(ctx, pid, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != StatusExpired {
		t.Fatalf("expected auto-expired offer, got %+v", items)
	}
}

func TestExpireSkipsTerminal(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	s.UpsertOffers(ctx, []api.Offer{sampleOffer("won", time.Now().Add(-time.Hour))})
	s.AddTracked(ctx, "won", pid)
	s.SetStatus(ctx, "won", pid, StatusOffer)

	items, _ := s.ListTracked(ctx, pid, "")
	if items[0].Status != StatusOffer {
		t.Errorf("terminal status should not expire, got %q", items[0].Status)
	}
}

func TestEvaluations(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	s.UpsertOffers(ctx, []api.Offer{sampleOffer("k1", time.Now().Add(24*time.Hour))})

	_, err := s.SaveEvaluation(ctx, Evaluation{
		OfferKey:   "k1",
		ProfileID:  pid,
		Grade:      "B",
		Dimensions: map[string]any{"skillMatch": "A"},
		Rationale:  "good fit",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LatestEvaluation(ctx, "k1", pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Grade != "B" || got.Dimensions["skillMatch"] != "A" {
		t.Errorf("eval roundtrip mismatch: %+v", got)
	}

	// Grade should surface in the tracker listing.
	s.AddTracked(ctx, "k1", pid)
	items, _ := s.ListTracked(ctx, pid, "")
	if items[0].Grade != "B" {
		t.Errorf("tracker grade = %q, want B", items[0].Grade)
	}
}

func TestSeenDiffing(t *testing.T) {
	s := openTemp(t)
	keys := []string{"a", "b", "c"}

	added, err := s.MarkSeen(ctx, keys)
	if err != nil {
		t.Fatal(err)
	}
	if added != 3 {
		t.Errorf("first MarkSeen added %d, want 3", added)
	}
	added, _ = s.MarkSeen(ctx, append(keys, "d"))
	if added != 1 {
		t.Errorf("second MarkSeen added %d, want 1 (only d is new)", added)
	}
	if ok, _ := s.IsSeen(ctx, "a"); !ok {
		t.Error("a should be seen")
	}
	if ok, _ := s.IsSeen(ctx, "z"); ok {
		t.Error("z should not be seen")
	}
}

func TestWatches(t *testing.T) {
	s := openTemp(t)
	if _, err := s.AddWatch(ctx, "go-roles", "IT", `{"searchTerms":["golang"]}`); err != nil {
		t.Fatal(err)
	}
	// Replace by name.
	if _, err := s.AddWatch(ctx, "go-roles", "IT", `{"minimumSalary":25000}`); err != nil {
		t.Fatal(err)
	}
	ws, _ := s.ListWatches(ctx)
	if len(ws) != 1 {
		t.Fatalf("expected 1 watch after replace, got %d", len(ws))
	}
	if ws[0].Params != `{"minimumSalary":25000}` {
		t.Errorf("watch params not replaced: %s", ws[0].Params)
	}
}

func TestProfiles(t *testing.T) {
	s := openTemp(t)
	p, err := s.AddProfile(ctx, "devops", "content here", true)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsDefault {
		t.Error("new profile should be default")
	}
	def, _ := s.DefaultProfile(ctx)
	if def.Name != "devops" {
		t.Errorf("default profile = %q, want devops", def.Name)
	}
	// The seeded 'default' profile should no longer be the default.
	old, _ := s.GetProfileByName(ctx, DefaultProfileName)
	if old.IsDefault {
		t.Error("old default should have been cleared")
	}
}

func mustDefaultID(t *testing.T, s *Store) int64 {
	t.Helper()
	p, err := s.DefaultProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}
