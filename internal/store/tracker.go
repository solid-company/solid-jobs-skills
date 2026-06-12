package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

// Tracked statuses for an application.
const (
	StatusSaved     = "saved"
	StatusApplied   = "applied"
	StatusInterview = "interview"
	StatusOffer     = "offer"
	StatusRejected  = "rejected"
	StatusExpired   = "expired"
)

// validStatuses is the set of statuses a user may set explicitly. "expired" is
// set automatically and is therefore allowed too.
var validStatuses = map[string]bool{
	StatusSaved: true, StatusApplied: true, StatusInterview: true,
	StatusOffer: true, StatusRejected: true, StatusExpired: true,
}

// ValidStatus reports whether s is a known tracker status.
func ValidStatus(s string) bool { return validStatuses[s] }

// TrackedOffer is a tracker row joined with display fields from the offer and
// its latest evaluation grade.
type TrackedOffer struct {
	OfferKey  string `json:"offerKey"`
	ProfileID int64  `json:"profileId"`
	Status    string `json:"status"`
	Notes     string `json:"notes"`
	Title     string `json:"title"`
	Company   string `json:"company"`
	URL       string `json:"url"`
	ValidTo   string `json:"validTo"`
	Grade     string `json:"grade,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// AddTracked starts tracking an offer for a profile (idempotent: keeps existing
// status if already tracked). The offer must already be cached.
func (s *Store) AddTracked(offerKey string, profileID int64) error {
	ok, err := s.OfferExists(offerKey)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: offer %q not in cache (search for it first)", ErrNotFound, offerKey)
	}
	ts := now()
	return s.q.InsertTracked(bg(), sqlcgen.InsertTrackedParams{
		OfferKey: offerKey, ProfileID: profileID, Status: StatusSaved,
		CreatedAt: ts, UpdatedAt: ts,
	})
}

// SetStatus updates the status of a tracked offer.
func (s *Store) SetStatus(offerKey string, profileID int64, status string) error {
	if !ValidStatus(status) {
		return fmt.Errorf("invalid status %q", status)
	}
	n, err := s.q.SetStatus(bg(), sqlcgen.SetStatusParams{
		Status: status, UpdatedAt: now(), OfferKey: offerKey, ProfileID: profileID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetNotes replaces the notes on a tracked offer.
func (s *Store) SetNotes(offerKey string, profileID int64, notes string) error {
	n, err := s.q.SetNotes(bg(), sqlcgen.SetNotesParams{
		Notes: notes, UpdatedAt: now(), OfferKey: offerKey, ProfileID: profileID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveTracked stops tracking an offer for a profile.
func (s *Store) RemoveTracked(offerKey string, profileID int64) error {
	n, err := s.q.RemoveTracked(bg(), sqlcgen.RemoveTrackedParams{
		OfferKey: offerKey, ProfileID: profileID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ExpireStale marks tracked offers as expired when their valid_to has passed,
// except those already in a terminal state (offer/rejected). Returns the count
// updated. Call before listing so the pipeline reflects reality.
func (s *Store) ExpireStale(profileID int64) (int64, error) {
	return s.q.ExpireStale(bg(), sqlcgen.ExpireStaleParams{
		UpdatedAt: now(),
		ProfileID: profileID,
		Cutoff:    nullStr(time.Now().UTC().Format(time.RFC3339)),
	})
}

// ListTracked returns tracked offers for a profile, optionally filtered by
// status, newest first. It expires stale rows first.
func (s *Store) ListTracked(profileID int64, status string) ([]TrackedOffer, error) {
	if _, err := s.ExpireStale(profileID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListTracked(bg(), sqlcgen.ListTrackedParams{
		ProfileID: profileID,
		Status:    nullStr(status),
	})
	if err != nil {
		return nil, err
	}
	out := make([]TrackedOffer, 0, len(rows))
	for _, r := range rows {
		out = append(out, TrackedOffer{
			OfferKey:  r.OfferKey,
			ProfileID: r.ProfileID,
			Status:    r.Status,
			Notes:     r.Notes,
			Title:     r.Title,
			Company:   r.Company,
			URL:       strOrEmpty(r.Url),
			ValidTo:   strOrEmpty(r.ValidTo),
			Grade:     anyToStr(r.Grade),
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

// GetTracked returns one tracker row.
func (s *Store) GetTracked(offerKey string, profileID int64) (*TrackedOffer, error) {
	r, err := s.q.GetTracked(bg(), sqlcgen.GetTrackedParams{
		OfferKey: offerKey, ProfileID: profileID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &TrackedOffer{
		OfferKey:  r.OfferKey,
		ProfileID: r.ProfileID,
		Status:    r.Status,
		Notes:     r.Notes,
		Title:     r.Title,
		Company:   r.Company,
		URL:       strOrEmpty(r.Url),
		ValidTo:   strOrEmpty(r.ValidTo),
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}, nil
}

// anyToStr coerces a COALESCE result (string, []byte or nil) to a string.
func anyToStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}
