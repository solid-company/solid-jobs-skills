// Package jobs is the service layer: all business logic for solid-jobs-skills
// lives here, built on internal/api and internal/store. Both the CLI
// (cmd/sjctl) and any future dashboard depend on this package, never the other
// way around.
package jobs

import (
	"context"
	"encoding/json"

	"github.com/solid-company/solid-jobs-skills/internal/api"
	"github.com/solid-company/solid-jobs-skills/internal/stats"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

// Service bundles the API client and the store.
type Service struct {
	Store  *store.Store
	Client *api.Client
}

// New builds a Service from an open store and a campaign string.
func New(st *store.Store, campaign string) *Service {
	return &Service{Store: st, Client: api.NewClient(campaign)}
}

// SearchOffers queries the live API, caches the returned offers, and returns
// the response. Caching lets later track/evaluate calls reference offers by key
// without another API round-trip.
func (s *Service) SearchOffers(ctx context.Context, division string, p api.SearchParams) (*api.OffersResponse, error) {
	resp, err := s.Client.Search(ctx, division, p)
	if err != nil {
		return nil, err
	}
	if len(resp.Jobs) > 0 {
		if err := s.Store.UpsertOffers(ctx, resp.Jobs); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// ResolveProfileID returns the id of the named profile, or the default profile
// when name is empty.
func (s *Service) ResolveProfileID(ctx context.Context, name string) (int64, error) {
	if name == "" {
		p, err := s.Store.DefaultProfile(ctx)
		if err != nil {
			return 0, err
		}
		return p.ID, nil
	}
	p, err := s.Store.GetProfileByName(ctx, name)
	if err != nil {
		return 0, err
	}
	return p.ID, nil
}

// SalaryStats computes a market snapshot over cached offers.
func (s *Service) SalaryStats(ctx context.Context, f stats.Filter) (*stats.Result, error) {
	return stats.Compute(ctx, s.Store.Queries(), f)
}

// MarketStatistics fetches server-side market statistics for a scope from the
// public API. Unlike SalaryStats (which aggregates the local cache), this
// reflects the whole live market for the given division/category/city/etc.
func (s *Service) MarketStatistics(ctx context.Context, scopeKind, scopeKey string, fields []string) (*api.MarketStats, error) {
	return s.Client.MarketStatistics(ctx, scopeKind, scopeKey, fields)
}

// MarketRaport fetches the live yearly market report for a single role.
func (s *Service) MarketRaport(ctx context.Context, scopeKey string) (*api.MarketRaport, error) {
	return s.Client.MarketRaport(ctx, scopeKey)
}

// NewOffer pairs a fresh offer with the watch that surfaced it.
type NewOffer struct {
	Watch string    `json:"watch"`
	Offer api.Offer `json:"offer"`
}

// SyncResult reports what a sync run found.
type SyncResult struct {
	WatchesRun int        `json:"watchesRun"`
	TotalSeen  int        `json:"totalSeen"`
	New        []NewOffer `json:"new"`
}

// RunSync executes every saved watch, caches results, and returns offers not
// previously reported. Newly reported keys are recorded in the seen set so a
// second run surfaces nothing new.
func (s *Service) RunSync(ctx context.Context) (*SyncResult, error) {
	watches, err := s.Store.ListWatches(ctx)
	if err != nil {
		return nil, err
	}
	out := &SyncResult{}
	for _, w := range watches {
		var p api.SearchParams
		if w.Params != "" {
			if err := json.Unmarshal([]byte(w.Params), &p); err != nil {
				return nil, err
			}
		}
		resp, err := s.SearchOffers(ctx, w.Division, p)
		if err != nil {
			return nil, err
		}
		out.WatchesRun++
		out.TotalSeen += len(resp.Jobs)

		keys := make([]string, len(resp.Jobs))
		for i := range resp.Jobs {
			keys[i] = resp.Jobs[i].JobOfferKey
		}
		seen, err := s.Store.SeenKeys(ctx, keys)
		if err != nil {
			return nil, err
		}
		for i := range resp.Jobs {
			o := resp.Jobs[i]
			if !seen[o.JobOfferKey] {
				out.New = append(out.New, NewOffer{Watch: w.Name, Offer: o})
			}
		}
		if _, err := s.Store.MarkSeen(ctx, keys); err != nil {
			return nil, err
		}
	}
	return out, nil
}
