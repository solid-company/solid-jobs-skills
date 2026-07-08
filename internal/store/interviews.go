package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

// ValidReadiness reports whether n is an accepted readiness score (0..100).
func ValidReadiness(n int) bool { return n >= 0 && n <= 100 }

// ValidConfidence reports whether n is an accepted per-question confidence (0..5).
func ValidConfidence(n int) bool { return n >= 0 && n <= 5 }

// confidenceMax is the top of the per-question confidence scale; readiness is
// the mean confidence expressed as a 0..100 percentage of this.
const confidenceMax = 5

// Gap is one skill/topic the candidate is weaker on for a given offer.
type Gap struct {
	Skill    string `json:"skill"`
	Severity string `json:"severity"` // low | medium | high
	Note     string `json:"note"`
}

// InterviewQuestion is a single practise question inside a prep session.
type InterviewQuestion struct {
	ID             int64  `json:"id"`
	PrepID         int64  `json:"prepId"`
	Category       string `json:"category"` // technical | behavioral | situational | company
	Question       string `json:"question"`
	Difficulty     string `json:"difficulty"` // easy | medium | hard
	TalkingPoints  string `json:"talkingPoints"`
	Confidence     int    `json:"confidence"` // 0..5
	PracticedCount int    `json:"practicedCount"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// InterviewPrep is a saved interview-preparation session for an (offer, profile).
type InterviewPrep struct {
	ID             int64               `json:"id"`
	OfferKey       string              `json:"offerKey"`
	ProfileID      int64               `json:"profileId"`
	Readiness      int                 `json:"readiness"` // 0..100
	Gaps           []Gap               `json:"gaps"`
	QuestionsToAsk []string            `json:"questionsToAsk"`
	Summary        string              `json:"summary"`
	Questions      []InterviewQuestion `json:"questions,omitempty"`
	CreatedAt      string              `json:"createdAt"`
	UpdatedAt      string              `json:"updatedAt"`
	// Title/Company/URL are populated by ListInterviewPreps for display.
	Title   string `json:"title,omitempty"`
	Company string `json:"company,omitempty"`
	URL     string `json:"url,omitempty"`
}

// SaveInterviewPrep records a new prep session together with its questions in a
// single transaction. The offer must be cached. History is preserved, so each
// call inserts a fresh session. The returned prep carries the assigned IDs.
func (s *Store) SaveInterviewPrep(ctx context.Context, p InterviewPrep) (*InterviewPrep, error) {
	ok, err := s.OfferExists(ctx, p.OfferKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: offer %q not in cache", ErrNotFound, p.OfferKey)
	}
	if !ValidReadiness(p.Readiness) {
		return nil, fmt.Errorf("readiness %d out of range (want 0-100)", p.Readiness)
	}

	gaps := p.Gaps
	if gaps == nil {
		gaps = []Gap{}
	}
	gapsJSON, err := json.Marshal(gaps)
	if err != nil {
		return nil, fmt.Errorf("marshal gaps: %w", err)
	}
	ask := p.QuestionsToAsk
	if ask == nil {
		ask = []string{}
	}
	askJSON, err := json.Marshal(ask)
	if err != nil {
		return nil, fmt.Errorf("marshal questionsToAsk: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	ts := now()
	prepRow, err := qtx.InsertInterviewPrep(ctx, sqlcgen.InsertInterviewPrepParams{
		OfferKey:       p.OfferKey,
		ProfileID:      p.ProfileID,
		Readiness:      int64(p.Readiness),
		Gaps:           string(gapsJSON),
		QuestionsToAsk: string(askJSON),
		Summary:        p.Summary,
		CreatedAt:      ts,
		UpdatedAt:      ts,
	})
	if err != nil {
		return nil, err
	}

	for i := range p.Questions {
		q := &p.Questions[i]
		difficulty := q.Difficulty
		if difficulty == "" {
			difficulty = "medium"
		}
		id, err := qtx.InsertInterviewQuestion(ctx, sqlcgen.InsertInterviewQuestionParams{
			PrepID:        prepRow.ID,
			Category:      q.Category,
			Question:      q.Question,
			Difficulty:    difficulty,
			TalkingPoints: q.TalkingPoints,
			CreatedAt:     ts,
			UpdatedAt:     ts,
		})
		if err != nil {
			return nil, err
		}
		q.ID = id
		q.PrepID = prepRow.ID
		q.Difficulty = difficulty
		q.CreatedAt = ts
		q.UpdatedAt = ts
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	p.ID = prepRow.ID
	p.CreatedAt = prepRow.CreatedAt
	p.UpdatedAt = prepRow.UpdatedAt
	p.Gaps = gaps
	p.QuestionsToAsk = ask
	return &p, nil
}

// LatestInterviewPrep returns the most recent prep for an offer+profile, without
// its questions (use QuestionsForPrep to load them).
func (s *Store) LatestInterviewPrep(ctx context.Context, offerKey string, profileID int64) (*InterviewPrep, error) {
	r, err := s.q.LatestInterviewPrep(ctx, sqlcgen.LatestInterviewPrepParams{
		OfferKey: offerKey, ProfileID: profileID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return prepFromRow(r)
}

// QuestionsForPrep returns every question belonging to a prep, ordered by
// category then insertion order.
func (s *Store) QuestionsForPrep(ctx context.Context, prepID int64) ([]InterviewQuestion, error) {
	rows, err := s.q.QuestionsForPrep(ctx, prepID)
	if err != nil {
		return nil, err
	}
	out := make([]InterviewQuestion, len(rows))
	for i, r := range rows {
		out[i] = questionFromModel(r)
	}
	return out, nil
}

// ListInterviewPreps returns the latest-updated prep sessions for a profile,
// each decorated with the offer's title/company/url for display.
func (s *Store) ListInterviewPreps(ctx context.Context, profileID int64) ([]InterviewPrep, error) {
	rows, err := s.q.ListInterviewPreps(ctx, profileID)
	if err != nil {
		return nil, err
	}
	out := make([]InterviewPrep, 0, len(rows))
	for _, r := range rows {
		p, err := prepFromRow(sqlcgen.InterviewPrep{
			ID:             r.ID,
			OfferKey:       r.OfferKey,
			ProfileID:      r.ProfileID,
			Readiness:      r.Readiness,
			Gaps:           r.Gaps,
			QuestionsToAsk: r.QuestionsToAsk,
			Summary:        r.Summary,
			CreatedAt:      r.CreatedAt,
			UpdatedAt:      r.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		p.Title = r.Title
		p.Company = r.Company
		p.URL = strOrEmpty(r.Url)
		out = append(out, *p)
	}
	return out, nil
}

// NextPracticeQuestions returns up to limit questions from the latest prep with
// the lowest confidence first — the ones the candidate should drill next.
func (s *Store) NextPracticeQuestions(ctx context.Context, offerKey string, profileID int64, limit int) ([]InterviewQuestion, error) {
	prep, err := s.LatestInterviewPrep(ctx, offerKey, profileID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.q.NextPracticeQuestions(ctx, sqlcgen.NextPracticeQuestionsParams{
		PrepID: prep.ID, Limit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]InterviewQuestion, len(rows))
	for i, r := range rows {
		out[i] = questionFromModel(r)
	}
	return out, nil
}

// RateQuestion records a self-rated confidence (0..5) for a question after
// practising it and recomputes the parent prep's readiness (mean confidence as
// a 0..100 percentage), all in one transaction. It returns the updated prep.
func (s *Store) RateQuestion(ctx context.Context, questionID int64, confidence int) (*InterviewPrep, error) {
	if !ValidConfidence(confidence) {
		return nil, fmt.Errorf("confidence %d out of range (want 0-5)", confidence)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	q, err := qtx.GetQuestion(ctx, questionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: question %d", ErrNotFound, questionID)
	}
	if err != nil {
		return nil, err
	}

	ts := now()
	if _, err := qtx.UpdateQuestionConfidence(ctx, sqlcgen.UpdateQuestionConfidenceParams{
		Confidence: int64(confidence), UpdatedAt: ts, ID: questionID,
	}); err != nil {
		return nil, err
	}

	qs, err := qtx.QuestionsForPrep(ctx, q.PrepID)
	if err != nil {
		return nil, err
	}
	readiness := computeReadiness(qs)
	if _, err := qtx.UpdatePrepReadiness(ctx, sqlcgen.UpdatePrepReadinessParams{
		Readiness: int64(readiness), UpdatedAt: ts, ID: q.PrepID,
	}); err != nil {
		return nil, err
	}

	prepRow, err := qtx.GetInterviewPrep(ctx, q.PrepID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return prepFromRow(prepRow)
}

// computeReadiness maps the mean question confidence onto a 0..100 score.
func computeReadiness(qs []sqlcgen.InterviewQuestion) int {
	if len(qs) == 0 {
		return 0
	}
	var sum int
	for _, q := range qs {
		sum += int(q.Confidence)
	}
	return int(math.Round(float64(sum) / float64(len(qs)*confidenceMax) * 100))
}

func prepFromRow(r sqlcgen.InterviewPrep) (*InterviewPrep, error) {
	p := InterviewPrep{
		ID:        r.ID,
		OfferKey:  r.OfferKey,
		ProfileID: r.ProfileID,
		Readiness: int(r.Readiness),
		Summary:   r.Summary,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
	if r.Gaps != "" {
		if err := json.Unmarshal([]byte(r.Gaps), &p.Gaps); err != nil {
			return nil, fmt.Errorf("decode prep gaps for offer %q: %w", r.OfferKey, err)
		}
	}
	if r.QuestionsToAsk != "" {
		if err := json.Unmarshal([]byte(r.QuestionsToAsk), &p.QuestionsToAsk); err != nil {
			return nil, fmt.Errorf("decode prep questionsToAsk for offer %q: %w", r.OfferKey, err)
		}
	}
	return &p, nil
}

func questionFromModel(r sqlcgen.InterviewQuestion) InterviewQuestion {
	return InterviewQuestion{
		ID:             r.ID,
		PrepID:         r.PrepID,
		Category:       r.Category,
		Question:       r.Question,
		Difficulty:     r.Difficulty,
		TalkingPoints:  r.TalkingPoints,
		Confidence:     int(r.Confidence),
		PracticedCount: int(r.PracticedCount),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}
