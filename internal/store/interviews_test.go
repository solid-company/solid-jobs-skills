package store

import (
	"errors"
	"testing"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

// upsertOffer caches one offer, failing the test on error.
func upsertOffer(t *testing.T, s *Store, o api.Offer) {
	t.Helper()
	if err := s.UpsertOffers(ctx, []api.Offer{o}); err != nil {
		t.Fatal(err)
	}
}

func TestInterviewPrepRoundtrip(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	upsertOffer(t, s, sampleOffer("k1", time.Now().Add(24*time.Hour)))

	saved, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{
		OfferKey:       "k1",
		ProfileID:      pid,
		Readiness:      40,
		Gaps:           []Gap{{Skill: "Kubernetes", Severity: "high", Note: "no prod experience"}},
		QuestionsToAsk: []string{"How is on-call handled?"},
		Summary:        "Strong Go, weak k8s",
		Questions: []InterviewQuestionInput{
			{Category: "technical", Question: "Explain a goroutine leak", Difficulty: "medium", TalkingPoints: "channels, context"},
			{Category: "behavioral", Question: "Tell me about a conflict"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || len(saved.Questions) != 2 || saved.Questions[0].ID == 0 {
		t.Fatalf("save did not assign ids: %+v", saved)
	}
	// A blank difficulty defaults to medium.
	if saved.Questions[1].Difficulty != "medium" {
		t.Errorf("difficulty default = %q, want medium", saved.Questions[1].Difficulty)
	}

	got, err := s.LatestInterviewPrep(ctx, "k1", pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Readiness != 40 || len(got.Gaps) != 1 || got.Gaps[0].Skill != "Kubernetes" {
		t.Errorf("prep roundtrip mismatch: %+v", got)
	}
	if len(got.QuestionsToAsk) != 1 || got.QuestionsToAsk[0] != "How is on-call handled?" {
		t.Errorf("questionsToAsk roundtrip mismatch: %+v", got.QuestionsToAsk)
	}

	qs, err := s.QuestionsForPrep(ctx, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("QuestionsForPrep = %d, want 2", len(qs))
	}
}

func TestSaveInterviewPrepRejectsBadEnums(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	upsertOffer(t, s, sampleOffer("k1", time.Now().Add(24*time.Hour)))

	cases := []struct {
		name string
		in   InterviewPrepInput
	}{
		{"bad category", InterviewPrepInput{OfferKey: "k1", ProfileID: pid,
			Questions: []InterviewQuestionInput{{Category: "tecnical", Question: "Q"}}}},
		{"bad difficulty", InterviewPrepInput{OfferKey: "k1", ProfileID: pid,
			Questions: []InterviewQuestionInput{{Category: "technical", Question: "Q", Difficulty: "insane"}}}},
		{"empty question", InterviewPrepInput{OfferKey: "k1", ProfileID: pid,
			Questions: []InterviewQuestionInput{{Category: "technical", Question: ""}}}},
		{"bad severity", InterviewPrepInput{OfferKey: "k1", ProfileID: pid,
			Gaps: []Gap{{Skill: "k8s", Severity: "critical"}}}},
		{"empty skill", InterviewPrepInput{OfferKey: "k1", ProfileID: pid,
			Gaps: []Gap{{Skill: "", Severity: "low"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.SaveInterviewPrep(ctx, tc.in); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}

	// Nothing above should have been persisted.
	if _, err := s.LatestInterviewPrep(ctx, "k1", pid); !errors.Is(err, ErrNotFound) {
		t.Errorf("bad saves leaked a prep: %v", err)
	}
}

func TestRateQuestionRecomputesReadiness(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	upsertOffer(t, s, sampleOffer("k1", time.Now().Add(24*time.Hour)))

	saved, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{
		OfferKey:  "k1",
		ProfileID: pid,
		Readiness: 0,
		Questions: []InterviewQuestionInput{
			{Category: "technical", Question: "Q1"},
			{Category: "technical", Question: "Q2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Rate the first question 5/5: mean confidence = 2.5/5 -> readiness 50.
	prep, err := s.RateQuestion(ctx, saved.Questions[0].ID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if prep.Readiness != 50 {
		t.Errorf("readiness after one 5/5 of two = %d, want 50", prep.Readiness)
	}

	// Rate the second 3/5: mean (5+3)/2 = 4 -> readiness 80.
	prep, err = s.RateQuestion(ctx, saved.Questions[1].ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if prep.Readiness != 80 {
		t.Errorf("readiness after 5 and 3 = %d, want 80", prep.Readiness)
	}

	// practiced_count increments and confidence persists.
	qs, err := s.QuestionsForPrep(ctx, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if qs[0].PracticedCount != 1 || qs[0].Confidence != 5 {
		t.Errorf("question state after rate: %+v", qs[0])
	}
}

func TestRateQuestionValidation(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	upsertOffer(t, s, sampleOffer("k1", time.Now().Add(24*time.Hour)))
	saved, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{
		OfferKey: "k1", ProfileID: pid,
		Questions: []InterviewQuestionInput{{Category: "technical", Question: "Q1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.RateQuestion(ctx, saved.Questions[0].ID, 9); err == nil {
		t.Error("expected error for out-of-range confidence")
	}
	if _, err := s.RateQuestion(ctx, 99999, 3); !errors.Is(err, ErrNotFound) {
		t.Errorf("rating a missing question: got %v, want ErrNotFound", err)
	}
}

func TestSaveInterviewPrepRequiresCachedOffer(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	if _, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{OfferKey: "ghost", ProfileID: pid}); !errors.Is(err, ErrNotFound) {
		t.Errorf("saving prep for uncached offer: got %v, want ErrNotFound", err)
	}
}

func TestLatestInterviewPrepNotFound(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	if _, err := s.LatestInterviewPrep(ctx, "nope", pid); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// Two preps for the same offer created in the same second must resolve to the
// newest by id, not an arbitrary row — this is what the created_at DESC, id DESC
// tiebreaker guarantees.
func TestLatestInterviewPrepPrefersNewest(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	upsertOffer(t, s, sampleOffer("k1", time.Now().Add(24*time.Hour)))

	if _, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{
		OfferKey: "k1", ProfileID: pid, Readiness: 30, Summary: "first",
	}); err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{
		OfferKey: "k1", ProfileID: pid, Readiness: 70, Summary: "second",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.LatestInterviewPrep(ctx, "k1", pid)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != second.ID || got.Readiness != 70 || got.Summary != "second" {
		t.Errorf("latest prep = %+v, want the second save (id %d, readiness 70)", got, second.ID)
	}
}

func TestNextPracticeQuestionsOrdering(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	upsertOffer(t, s, sampleOffer("k1", time.Now().Add(24*time.Hour)))

	saved, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{
		OfferKey: "k1", ProfileID: pid,
		Questions: []InterviewQuestionInput{
			{Category: "technical", Question: "Q1"},
			{Category: "technical", Question: "Q2"},
			{Category: "technical", Question: "Q3"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Q1 gets high confidence (and a practice count); Q2 and Q3 stay unrated.
	if _, err := s.RateQuestion(ctx, saved.Questions[0].ID, 5); err != nil {
		t.Fatal(err)
	}

	qs, err := s.NextPracticeQuestions(ctx, "k1", pid, 0) // limit 0 -> default
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 3 {
		t.Fatalf("got %d questions, want 3", len(qs))
	}
	// Lowest confidence first; ties broken by practiced_count then id. So the two
	// unrated questions (Q2, Q3 in id order) precede the rated Q1.
	want := []int64{saved.Questions[1].ID, saved.Questions[2].ID, saved.Questions[0].ID}
	for i, id := range want {
		if qs[i].ID != id {
			t.Errorf("position %d: got question #%d, want #%d (order %+v)", i,
				qs[i].ID, id, []int64{qs[0].ID, qs[1].ID, qs[2].ID})
		}
	}
}

func TestNextPracticeQuestionsDefaultLimit(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	upsertOffer(t, s, sampleOffer("k1", time.Now().Add(24*time.Hour)))

	in := InterviewPrepInput{OfferKey: "k1", ProfileID: pid}
	for i := 0; i < 7; i++ {
		in.Questions = append(in.Questions, InterviewQuestionInput{Category: "technical", Question: "Q"})
	}
	if _, err := s.SaveInterviewPrep(ctx, in); err != nil {
		t.Fatal(err)
	}

	qs, err := s.NextPracticeQuestions(ctx, "k1", pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 5 {
		t.Errorf("default limit returned %d questions, want 5", len(qs))
	}
}

func TestListInterviewPrepsDedupAndDecorate(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)

	o1 := sampleOffer("k1", time.Now().Add(24*time.Hour))
	o1.Title, o1.Company = "Go Engineer", "Acme"
	o2 := sampleOffer("k2", time.Now().Add(24*time.Hour))
	o2.Title, o2.Company = "Rust Engineer", "Globex"
	upsertOffer(t, s, o1)
	upsertOffer(t, s, o2)

	// k1, then k2, then re-prep k1 (its latest should win, and appear once).
	if _, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{OfferKey: "k1", ProfileID: pid, Readiness: 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{OfferKey: "k2", ProfileID: pid, Readiness: 55}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveInterviewPrep(ctx, InterviewPrepInput{OfferKey: "k1", ProfileID: pid, Readiness: 90}); err != nil {
		t.Fatal(err)
	}

	preps, err := s.ListInterviewPreps(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(preps) != 2 {
		t.Fatalf("got %d preps, want 2 (one per offer)", len(preps))
	}
	// Newest write first (k1's re-prep), decorated from the offer.
	if preps[0].OfferKey != "k1" || preps[0].Readiness != 90 {
		t.Errorf("first prep = %+v, want k1 with latest readiness 90", preps[0])
	}
	if preps[0].Title != "Go Engineer" || preps[0].Company != "Acme" {
		t.Errorf("k1 decoration mismatch: %+v", preps[0])
	}
	if preps[1].OfferKey != "k2" || preps[1].Title != "Rust Engineer" || preps[1].Company != "Globex" {
		t.Errorf("second prep = %+v, want decorated k2", preps[1])
	}
}
