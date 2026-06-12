package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

// validGrades is the set of overall grades an evaluation may carry (A best, F
// worst), matching what the jobs-evaluate skill produces.
var validGrades = map[string]bool{
	"A": true, "B": true, "C": true, "D": true, "E": true, "F": true,
}

// ValidGrade reports whether g is an accepted A–F grade.
func ValidGrade(g string) bool { return validGrades[g] }

// Evaluation is an AI scoring of an offer against a profile.
type Evaluation struct {
	ID         int64          `json:"id"`
	OfferKey   string         `json:"offerKey"`
	ProfileID  int64          `json:"profileId"`
	Grade      string         `json:"grade"`
	Dimensions map[string]any `json:"dimensions"`
	Rationale  string         `json:"rationale"`
	CreatedAt  string         `json:"createdAt"`
}

// SaveEvaluation records a new evaluation row (history is preserved). The offer
// must be cached.
func (s *Store) SaveEvaluation(ctx context.Context, e Evaluation) (*Evaluation, error) {
	ok, err := s.OfferExists(ctx, e.OfferKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: offer %q not in cache", ErrNotFound, e.OfferKey)
	}
	dims := e.Dimensions
	if dims == nil {
		dims = map[string]any{}
	}
	dimJSON, err := json.Marshal(dims)
	if err != nil {
		return nil, fmt.Errorf("marshal dimensions: %w", err)
	}
	row, err := s.q.InsertEvaluation(ctx, sqlcgen.InsertEvaluationParams{
		OfferKey:   e.OfferKey,
		ProfileID:  e.ProfileID,
		Grade:      e.Grade,
		Dimensions: string(dimJSON),
		Rationale:  e.Rationale,
		CreatedAt:  now(),
	})
	if err != nil {
		return nil, err
	}
	e.ID = row.ID
	e.CreatedAt = row.CreatedAt
	e.Dimensions = dims
	return &e, nil
}

// LatestEvaluation returns the most recent evaluation for an offer+profile.
func (s *Store) LatestEvaluation(ctx context.Context, offerKey string, profileID int64) (*Evaluation, error) {
	r, err := s.q.LatestEvaluation(ctx, sqlcgen.LatestEvaluationParams{
		OfferKey: offerKey, ProfileID: profileID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e := Evaluation{
		ID:        r.ID,
		OfferKey:  r.OfferKey,
		ProfileID: r.ProfileID,
		Grade:     r.Grade,
		Rationale: r.Rationale,
		CreatedAt: r.CreatedAt,
	}
	if r.Dimensions != "" {
		if err := json.Unmarshal([]byte(r.Dimensions), &e.Dimensions); err != nil {
			return nil, fmt.Errorf("decode evaluation dimensions for offer %q: %w", offerKey, err)
		}
	}
	return &e, nil
}
