package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

// Profile is a user CV/preferences profile.
type Profile struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	IsDefault bool   `json:"isDefault"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// AddProfile creates a new profile. If makeDefault is true it becomes the
// default (clearing the flag on all others).
func (s *Store) AddProfile(ctx context.Context, name, content string, makeDefault bool) (*Profile, error) {
	if name == "" {
		return nil, errors.New("profile name required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	ts := now()
	id, err := qtx.InsertProfile(ctx, sqlcgen.InsertProfileParams{
		Name: name, Content: content, CreatedAt: ts, UpdatedAt: ts,
	})
	if err != nil {
		return nil, fmt.Errorf("insert profile: %w", err)
	}
	if makeDefault {
		if err := qtx.SetDefaultProfile(ctx, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, id)
}

// SetProfileContent replaces a profile's content (e.g. imported from profile.md).
func (s *Store) SetProfileContent(ctx context.Context, id int64, content string) error {
	n, err := s.q.SetProfileContent(ctx, sqlcgen.SetProfileContentParams{
		Content: content, UpdatedAt: now(), ID: id,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDefaultProfile marks a single profile as the default, clearing the flag on
// all others.
func (s *Store) SetDefaultProfile(ctx context.Context, id int64) error {
	exists, err := s.q.ProfileExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return s.q.SetDefaultProfile(ctx, id)
}

// GetProfile fetches a profile by id.
func (s *Store) GetProfile(ctx context.Context, id int64) (*Profile, error) {
	p, err := s.q.GetProfile(ctx, id)
	return toProfile(p, err)
}

// GetProfileByName fetches a profile by its unique name.
func (s *Store) GetProfileByName(ctx context.Context, name string) (*Profile, error) {
	p, err := s.q.GetProfileByName(ctx, name)
	return toProfile(p, err)
}

// DefaultProfile returns the current default profile.
func (s *Store) DefaultProfile(ctx context.Context) (*Profile, error) {
	p, err := s.q.DefaultProfile(ctx)
	return toProfile(p, err)
}

// ListProfiles returns all profiles, default first then by name.
func (s *Store) ListProfiles(ctx context.Context) ([]Profile, error) {
	rows, err := s.q.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(rows))
	for _, p := range rows {
		out = append(out, fromGen(p))
	}
	return out, nil
}

func toProfile(p sqlcgen.Profile, err error) (*Profile, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := fromGen(p)
	return &out, nil
}

func fromGen(p sqlcgen.Profile) Profile {
	return Profile{
		ID:        p.ID,
		Name:      p.Name,
		Content:   p.Content,
		IsDefault: p.IsDefault == 1,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
