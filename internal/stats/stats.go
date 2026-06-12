// Package stats computes salary and market aggregates over the cached offers
// table. The underlying query is sqlc-generated (SalaryRows); median and shares
// are computed in Go since SQLite lacks a built-in median.
package stats

import (
	"context"
	"database/sql"
	"sort"

	"github.com/solid-company/solid-jobs-skills/internal/store/sqlcgen"
)

// Filter narrows which cached offers are aggregated. Empty fields are ignored.
type Filter struct {
	Division string
	Skill    string // matched against the offer's raw JSON (skills/title)
	City     string // matched against the offer's raw JSON (locations)
	Remote   bool   // when true, only remote offers
}

// Result is the computed market snapshot.
type Result struct {
	OfferCount   int            `json:"offerCount"`
	WithSalary   int            `json:"withSalary"`
	SalaryMin    int            `json:"salaryMin"`
	SalaryMedian int            `json:"salaryMedian"`
	SalaryMax    int            `json:"salaryMax"`
	Currency     string         `json:"currency"`
	RemoteCount  int            `json:"remoteCount"`
	RemoteShare  float64        `json:"remoteShare"`
	ByExperience map[string]int `json:"byExperience"`
	ByDivision   map[string]int `json:"byDivision"`
}

// Compute runs the sqlc SalaryRows query and aggregates the results.
func Compute(ctx context.Context, q *sqlcgen.Queries, f Filter) (*Result, error) {
	rows, err := q.SalaryRows(ctx, toParams(f))
	if err != nil {
		return nil, err
	}

	res := &Result{ByExperience: map[string]int{}, ByDivision: map[string]int{}}
	var salaries []int
	currencyVotes := map[string]int{}

	for _, r := range rows {
		res.OfferCount++
		if r.IsRemote == 1 {
			res.RemoteCount++
		}
		if r.ExperienceLevel.Valid && r.ExperienceLevel.String != "" {
			res.ByExperience[r.ExperienceLevel.String]++
		}
		if r.Division != "" {
			res.ByDivision[r.Division]++
		}
		// Representative salary: midpoint when both bounds present, else whichever exists.
		if s, ok := midpoint(r.SalaryFrom, r.SalaryTo); ok {
			salaries = append(salaries, s)
			res.WithSalary++
			if r.Currency.Valid && r.Currency.String != "" {
				currencyVotes[r.Currency.String]++
			}
		}
	}

	if len(salaries) > 0 {
		sort.Ints(salaries)
		res.SalaryMin = salaries[0]
		res.SalaryMax = salaries[len(salaries)-1]
		res.SalaryMedian = median(salaries)
	}
	res.Currency = topKey(currencyVotes)
	if res.OfferCount > 0 {
		res.RemoteShare = float64(res.RemoteCount) / float64(res.OfferCount)
	}
	return res, nil
}

// toParams maps the Filter to the nullable sqlc params. LIKE filters get %…%
// wildcards; an empty field stays NULL and is ignored by the query.
func toParams(f Filter) sqlcgen.SalaryRowsParams {
	p := sqlcgen.SalaryRowsParams{}
	if f.Division != "" {
		p.Division = sql.NullString{String: f.Division, Valid: true}
	}
	if f.Remote {
		p.Remote = sql.NullInt64{Int64: 1, Valid: true}
	}
	if f.Skill != "" {
		p.Skill = sql.NullString{String: "%" + f.Skill + "%", Valid: true}
	}
	if f.City != "" {
		p.City = sql.NullString{String: "%" + f.City + "%", Valid: true}
	}
	return p
}

func midpoint(from, to sql.NullInt64) (int, bool) {
	switch {
	case from.Valid && to.Valid:
		return int((from.Int64 + to.Int64) / 2), true
	case from.Valid:
		return int(from.Int64), true
	case to.Valid:
		return int(to.Int64), true
	default:
		return 0, false
	}
}

func median(sorted []int) int {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func topKey(votes map[string]int) string {
	best, bestN := "", 0
	for k, n := range votes {
		if n > bestN {
			best, bestN = k, n
		}
	}
	return best
}
