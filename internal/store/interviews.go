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

// Accepted enum values, mirroring the tracker/evaluations ValidStatus/ValidGrade
// pattern. Free-text values are rejected at save time so a typo can't silently
// create its own group in the by-category orderings.
const (
	CategoryTechnical   = "technical"
	CategoryBehavioral  = "behavioral"
	CategorySituational = "situational"
	CategoryCompany     = "company"

	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"

	SeverityLow    = "low"
	SeverityMedium = "medium"
	SeverityHigh   = "high"
)

var (
	validCategories = map[string]bool{
		CategoryTechnical: true, CategoryBehavioral: true,
		CategorySituational: true, CategoryCompany: true,
	}
	validDifficulties = map[string]bool{
		DifficultyEasy: true, DifficultyMedium: true, DifficultyHard: true,
	}
	validSeverities = map[string]bool{
		SeverityLow: true, SeverityMedium: true, SeverityHigh: true,
	}
)

// ValidCategory reports whether c is a known question category.
func ValidCategory(c string) bool { return validCategories[c] }

// ValidDifficulty reports whether d is a known question difficulty.
func ValidDifficulty(d string) bool { return validDifficulties[d] }

// ValidSeverity reports whether s is a known gap severity.
func ValidSeverity(s string) bool { return validSeverities[s] }

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
}

// InterviewPrepListItem is one row of ListInterviewPreps: a prep summary
// decorated with the offer's display fields, following the TrackedOffer
// precedent of a dedicated list type. Gaps/questions are omitted — load the full
// prep with LatestInterviewPrep + QuestionsForPrep when the detail is needed.
type InterviewPrepListItem struct {
	OfferKey  string `json:"offerKey"`
	ProfileID int64  `json:"profileId"`
	Readiness int    `json:"readiness"`
	Summary   string `json:"summary"`
	Title     string `json:"title"`
	Company   string `json:"company"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// InterviewQuestionInput is the caller-supplied shape of a question at save time.
// It deliberately omits the server-assigned and mock-loop-managed fields (id,
// prepId, confidence, practicedCount, timestamps) so they can't be supplied on a
// save only to be silently dropped.
type InterviewQuestionInput struct {
	Category      string `json:"category"` // technical | behavioral | situational | company
	Question      string `json:"question"`
	Difficulty    string `json:"difficulty"` // easy | medium | hard (blank -> medium)
	TalkingPoints string `json:"talkingPoints"`
}

// InterviewPrepInput is the caller-supplied data for SaveInterviewPrep. Like
// InterviewQuestionInput it carries no server-assigned fields (ids, timestamps)
// and no computed readiness history.
type InterviewPrepInput struct {
	OfferKey       string
	ProfileID      int64
	Readiness      int
	Gaps           []Gap
	QuestionsToAsk []string
	Summary        string
	Questions      []InterviewQuestionInput
}

// SaveInterviewPrep records a new prep session together with its questions in a
// single transaction. The offer must be cached. History is preserved, so each
// call inserts a fresh session. Enum fields (gap severity, question category and
// difficulty) are validated so a typo can't silently create its own group. The
// returned prep carries the assigned IDs; its readiness is the caller's estimate
// at save time and is recomputed from confidence by RateQuestion.
func (s *Store) SaveInterviewPrep(ctx context.Context, in InterviewPrepInput) (*InterviewPrep, error) {
	ok, err := s.OfferExists(ctx, in.OfferKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: offer %q not in cache", ErrNotFound, in.OfferKey)
	}
	if !ValidReadiness(in.Readiness) {
		return nil, fmt.Errorf("readiness %d out of range (want 0-100)", in.Readiness)
	}

	gaps := in.Gaps
	if gaps == nil {
		gaps = []Gap{}
	}
	for i, g := range gaps {
		if g.Skill == "" {
			return nil, fmt.Errorf("gap %d: skill is required", i)
		}
		if !ValidSeverity(g.Severity) {
			return nil, fmt.Errorf("gap %d: invalid severity %q (want low|medium|high)", i, g.Severity)
		}
	}
	gapsJSON, err := json.Marshal(gaps)
	if err != nil {
		return nil, fmt.Errorf("marshal gaps: %w", err)
	}
	ask := in.QuestionsToAsk
	if ask == nil {
		ask = []string{}
	}
	askJSON, err := json.Marshal(ask)
	if err != nil {
		return nil, fmt.Errorf("marshal questionsToAsk: %w", err)
	}

	// Validate questions up front (and default blank difficulty to medium) so a
	// bad one fails before any row is written.
	questions := make([]InterviewQuestion, len(in.Questions))
	for i, q := range in.Questions {
		if q.Question == "" {
			return nil, fmt.Errorf("question %d: text is required", i)
		}
		if !ValidCategory(q.Category) {
			return nil, fmt.Errorf("question %d: invalid category %q (want technical|behavioral|situational|company)", i, q.Category)
		}
		difficulty := q.Difficulty
		if difficulty == "" {
			difficulty = DifficultyMedium
		}
		if !ValidDifficulty(difficulty) {
			return nil, fmt.Errorf("question %d: invalid difficulty %q (want easy|medium|hard)", i, q.Difficulty)
		}
		questions[i] = InterviewQuestion{
			Category:      q.Category,
			Question:      q.Question,
			Difficulty:    difficulty,
			TalkingPoints: q.TalkingPoints,
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	ts := now()
	prepRow, err := qtx.InsertInterviewPrep(ctx, sqlcgen.InsertInterviewPrepParams{
		OfferKey:       in.OfferKey,
		ProfileID:      in.ProfileID,
		Readiness:      int64(in.Readiness),
		Gaps:           string(gapsJSON),
		QuestionsToAsk: string(askJSON),
		Summary:        in.Summary,
		CreatedAt:      ts,
		UpdatedAt:      ts,
	})
	if err != nil {
		return nil, err
	}

	for i := range questions {
		q := &questions[i]
		id, err := qtx.InsertInterviewQuestion(ctx, sqlcgen.InsertInterviewQuestionParams{
			PrepID:        prepRow.ID,
			Category:      q.Category,
			Question:      q.Question,
			Difficulty:    q.Difficulty,
			TalkingPoints: q.TalkingPoints,
			CreatedAt:     ts,
			UpdatedAt:     ts,
		})
		if err != nil {
			return nil, err
		}
		q.ID = id
		q.PrepID = prepRow.ID
		q.CreatedAt = ts
		q.UpdatedAt = ts
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &InterviewPrep{
		ID:             prepRow.ID,
		OfferKey:       in.OfferKey,
		ProfileID:      in.ProfileID,
		Readiness:      in.Readiness,
		Gaps:           gaps,
		QuestionsToAsk: ask,
		Summary:        in.Summary,
		Questions:      questions,
		CreatedAt:      prepRow.CreatedAt,
		UpdatedAt:      prepRow.UpdatedAt,
	}, nil
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

// ListInterviewPreps returns the latest prep per offer for a profile (one row
// per offer even though history is preserved), each decorated with the offer's
// title/company/url for display and ordered most-recently-updated first.
func (s *Store) ListInterviewPreps(ctx context.Context, profileID int64) ([]InterviewPrepListItem, error) {
	rows, err := s.q.ListInterviewPreps(ctx, profileID)
	if err != nil {
		return nil, err
	}
	out := make([]InterviewPrepListItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, InterviewPrepListItem{
			OfferKey:  r.OfferKey,
			ProfileID: r.ProfileID,
			Readiness: int(r.Readiness),
			Summary:   r.Summary,
			Title:     r.Title,
			Company:   r.Company,
			URL:       strOrEmpty(r.Url),
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
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

// computeReadiness maps the mean question confidence onto a 0..100 score. It
// averages over ALL questions, counting not-yet-practised ones as confidence 0.
// So the score is a whole-bank mean, not a mean of only what's been drilled:
// readiness intentionally starts low and climbs as more questions are rated
// (the first rating on a large bank moves it only a little). This replaces the
// caller's save-time estimate once RateQuestion runs.
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
