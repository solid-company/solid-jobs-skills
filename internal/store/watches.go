package store

import (
	"database/sql"
	"errors"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

// Watch is a saved search that `sync` re-runs to find new offers.
type Watch struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Division  string `json:"division"`
	Params    string `json:"params"` // JSON-encoded api.SearchParams
	CreatedAt string `json:"createdAt"`
}

// AddWatch saves (or replaces by name) a watch definition.
func (s *Store) AddWatch(name, division, paramsJSON string) (*Watch, error) {
	if paramsJSON == "" {
		paramsJSON = "{}"
	}
	if err := s.q.UpsertWatch(bg(), sqlcgen.UpsertWatchParams{
		Name: name, Division: division, Params: paramsJSON, CreatedAt: now(),
	}); err != nil {
		return nil, err
	}
	return s.GetWatchByName(name)
}

// ListWatches returns all saved watches ordered by name.
func (s *Store) ListWatches() ([]Watch, error) {
	rows, err := s.q.ListWatches(bg())
	if err != nil {
		return nil, err
	}
	out := make([]Watch, 0, len(rows))
	for _, w := range rows {
		out = append(out, toWatch(w))
	}
	return out, nil
}

// GetWatchByName fetches a watch by its unique name.
func (s *Store) GetWatchByName(name string) (*Watch, error) {
	w, err := s.q.GetWatchByName(bg(), name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := toWatch(w)
	return &out, nil
}

// RemoveWatch deletes a watch by name.
func (s *Store) RemoveWatch(name string) error {
	n, err := s.q.RemoveWatch(bg(), name)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkSeen records offer keys as already reported by sync. Returns how many
// were newly inserted (i.e. genuinely new).
func (s *Store) MarkSeen(keys []string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	ts := now()
	added := 0
	for _, k := range keys {
		n, err := qtx.MarkSeen(bg(), sqlcgen.MarkSeenParams{OfferKey: k, SeenAt: ts})
		if err != nil {
			return 0, err
		}
		if n > 0 {
			added++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return added, nil
}

// IsSeen reports whether an offer key has already been reported by sync.
func (s *Store) IsSeen(key string) (bool, error) {
	return s.q.IsSeen(bg(), key)
}

func toWatch(w sqlcgen.Watch) Watch {
	return Watch{
		ID:        w.ID,
		Name:      w.Name,
		Division:  w.Division,
		Params:    w.Params,
		CreatedAt: w.CreatedAt,
	}
}
