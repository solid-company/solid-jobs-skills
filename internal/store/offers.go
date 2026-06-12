package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/api"
	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// UpsertOffers caches a batch of offers, replacing any existing rows by key and
// stamping them with the current fetch time.
func (s *Store) UpsertOffers(ctx context.Context, offers []api.Offer) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.q.WithTx(tx)
	ts := now()
	for i := range offers {
		o := &offers[i]
		raw, err := json.Marshal(o)
		if err != nil {
			return fmt.Errorf("marshal offer %s: %w", o.JobOfferKey, err)
		}
		var from, to sql.NullInt64
		var currency sql.NullString
		if o.Salary != nil {
			from, to = nullRound(o.Salary.From), nullRound(o.Salary.To)
			currency = nullStr(o.Salary.Currency)
		}
		if err := qtx.UpsertOffer(ctx, sqlcgen.UpsertOfferParams{
			OfferKey:        o.JobOfferKey,
			Title:           o.Title,
			Company:         o.Company,
			Division:        o.Division,
			Category:        nullStr(o.Category),
			SubCategory:     nullStr(o.SubCategory),
			ExperienceLevel: nullStr(o.ExperienceLevel),
			SalaryFrom:      from,
			SalaryTo:        to,
			Currency:        currency,
			IsRemote:        b2i(o.IsRemote),
			IsHybrid:        b2i(o.IsHybrid),
			ValidFrom:       nullStr(fmtTime(o.ValidFrom)),
			ValidTo:         nullStr(fmtTime(o.ValidTo)),
			Url:             nullStr(o.URL),
			Locations:       nullStr(strings.Join(o.Locations, "\n")),
			Skills:          nullStr(skillNames(o.Skills)),
			Raw:             string(raw),
			FetchedAt:       ts,
		}); err != nil {
			return fmt.Errorf("upsert offer %s: %w", o.JobOfferKey, err)
		}
	}
	return tx.Commit()
}

// GetOffer reconstructs a cached offer from its stored raw JSON.
func (s *Store) GetOffer(ctx context.Context, key string) (*api.Offer, error) {
	raw, err := s.q.GetOfferRaw(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var o api.Offer
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		return nil, fmt.Errorf("decode cached offer: %w", err)
	}
	return &o, nil
}

// OfferExists reports whether an offer with the given key is cached.
func (s *Store) OfferExists(ctx context.Context, key string) (bool, error) {
	return s.q.OfferExists(ctx, key)
}

// skillNames joins an offer's skill names with newlines so the skill filter can
// LIKE-match against real skill fields rather than the whole raw JSON.
func skillNames(skills []api.NamedLevel) string {
	if len(skills) == 0 {
		return ""
	}
	names := make([]string, len(skills))
	for i, sk := range skills {
		names[i] = sk.Name
	}
	return strings.Join(names, "\n")
}

// nullRound rounds a nullable float salary to a nullable integer for storage.
func nullRound(f *float64) sql.NullInt64 {
	if f == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*f + 0.5), Valid: true}
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
