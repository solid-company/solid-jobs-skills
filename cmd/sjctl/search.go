package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/api"
	"github.com/solid-company/solid-jobs-skills/internal/jobs"
)

func newSearchCmd() *cobra.Command {
	var (
		division      string
		pageIndex     int
		pageSize      int
		sortActive    string
		sortDirection string
		cities        []string
		categories    []string
		subCategories []string
		experiences   []string
		terms         []string
		minSalary     int
		remoteOnly    bool
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search live offers and cache them",
		Long: "Query the SOLID.Jobs API for offers in a division. Results are cached " +
			"locally so you can track or evaluate them by key afterwards.\n\n" +
			"Divisions: IT, Engineering, Marketing, Sales, HR, Logistics, Finances, Other.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !api.ValidDivision(division) {
				return fmt.Errorf("invalid division %q: valid: %v", division, api.Divisions)
			}
			p := api.SearchParams{
				PageIndex:     pageIndex,
				PageSize:      pageSize,
				SortActive:    sortActive,
				SortDirection: sortDirection,
				Cities:        cities,
				Categories:    categories,
				SubCategories: subCategories,
				Experiences:   experiences,
				SearchTerms:   terms,
				MinimumSalary: minSalary,
			}
			return withService(func(s *jobs.Service) error {
				resp, err := s.SearchOffers(cmd.Context(), division, p)
				if err != nil {
					return fail("search", err)
				}
				offers := resp.Jobs
				if remoteOnly {
					offers = filterRemote(offers)
				}
				if gf.jsonOut {
					out := map[string]any{
						"pageIndex": resp.PageIndex,
						"pageSize":  resp.PageSize,
						"jobs":      offers,
					}
					if remoteOnly {
						// Client-side filter: server totals don't describe this subset.
						out["filtered"] = "remote"
						out["shown"] = len(offers)
					} else {
						out["totalCount"] = resp.TotalCount
						out["totalPages"] = resp.TotalPages
					}
					return printJSON(out)
				}
				printOffersTable(offers)
				if remoteOnly {
					fmt.Printf("\n%d remote offers on this page (of %d fetched, page %d/%d)\n",
						len(offers), len(resp.Jobs), resp.PageIndex+1, max(resp.TotalPages, 1))
				} else {
					fmt.Printf("\n%d shown · %d total · page %d/%d\n",
						len(offers), resp.TotalCount, resp.PageIndex+1, max(resp.TotalPages, 1))
				}
				return nil
			})
		},
	}

	f := cmd.Flags()
	f.StringVarP(&division, "division", "d", "IT", "division to search")
	f.IntVar(&pageIndex, "page-index", 0, "zero-based page index")
	f.IntVar(&pageSize, "page-size", 30, "results per page (max 500)")
	f.StringVar(&sortActive, "sort", "", "sort field (validFrom, salaryFrom, title, ...)")
	f.StringVar(&sortDirection, "sort-dir", "", "sort direction (asc|desc)")
	f.StringSliceVar(&cities, "city", nil, "filter by city (repeatable)")
	f.StringSliceVar(&categories, "category", nil, "filter by category (e.g. Developer)")
	f.StringSliceVar(&subCategories, "subcategory", nil, "filter by subcategory (e.g. Java)")
	f.StringSliceVar(&experiences, "experience", nil, "filter by experience (e.g. Senior)")
	f.StringSliceVar(&terms, "term", nil, "full-text search term (repeatable)")
	f.IntVar(&minSalary, "min-salary", 0, "minimum salary threshold")
	f.BoolVar(&remoteOnly, "remote", false, "only remote offers (client-side filter)")
	return cmd
}

func filterRemote(in []api.Offer) []api.Offer {
	out := in[:0:0]
	for _, o := range in {
		if o.IsRemote {
			out = append(out, o)
		}
	}
	return out
}
