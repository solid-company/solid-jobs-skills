package store

import (
	"errors"
	"testing"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

func TestInterviewPrepRoundtrip(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	s.UpsertOffers(ctx, []api.Offer{sampleOffer("k1", time.Now().Add(24*time.Hour))})

	saved, err := s.SaveInterviewPrep(ctx, InterviewPrep{
		OfferKey:       "k1",
		ProfileID:      pid,
		Readiness:      40,
		Gaps:           []Gap{{Skill: "Kubernetes", Severity: "high", Note: "no prod experience"}},
		QuestionsToAsk: []string{"How is on-call handled?"},
		Summary:        "Strong Go, weak k8s",
		Questions: []InterviewQuestion{
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

func TestRateQuestionRecomputesReadiness(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	s.UpsertOffers(ctx, []api.Offer{sampleOffer("k1", time.Now().Add(24*time.Hour))})

	saved, err := s.SaveInterviewPrep(ctx, InterviewPrep{
		OfferKey:  "k1",
		ProfileID: pid,
		Readiness: 0,
		Questions: []InterviewQuestion{
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
	qs, _ := s.QuestionsForPrep(ctx, saved.ID)
	if qs[0].PracticedCount != 1 || qs[0].Confidence != 5 {
		t.Errorf("question state after rate: %+v", qs[0])
	}
}

func TestRateQuestionValidation(t *testing.T) {
	s := openTemp(t)
	pid := mustDefaultID(t, s)
	s.UpsertOffers(ctx, []api.Offer{sampleOffer("k1", time.Now().Add(24*time.Hour))})
	saved, _ := s.SaveInterviewPrep(ctx, InterviewPrep{
		OfferKey: "k1", ProfileID: pid,
		Questions: []InterviewQuestion{{Category: "technical", Question: "Q1"}},
	})

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
	if _, err := s.SaveInterviewPrep(ctx, InterviewPrep{OfferKey: "ghost", ProfileID: pid}); !errors.Is(err, ErrNotFound) {
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
