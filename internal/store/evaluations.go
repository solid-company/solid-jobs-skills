package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

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
func (s *Store) SaveEvaluation(e Evaluation) (*Evaluation, error) {
	ok, err := s.OfferExists(e.OfferKey)
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
	row, err := s.q.InsertEvaluation(bg(), sqlcgen.InsertEvaluationParams{
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
func (s *Store) LatestEvaluation(offerKey string, profileID int64) (*Evaluation, error) {
	r, err := s.q.LatestEvaluation(bg(), sqlcgen.LatestEvaluationParams{
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
		_ = json.Unmarshal([]byte(r.Dimensions), &e.Dimensions)
	}
	return &e, nil
}
