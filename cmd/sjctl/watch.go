package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/api"
	"github.com/solid-company/solid-jobs-skills/internal/jobs"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage saved searches for sync/digest",
	}
	cmd.AddCommand(newWatchAddCmd(), newWatchListCmd(), newWatchRmCmd())
	return cmd
}

func newWatchAddCmd() *cobra.Command {
	var (
		division   string
		cities     []string
		categories []string
		terms      []string
		experience []string
		minSalary  int
		pageSize   int
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Save a search as a watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !api.ValidDivision(division) {
				return fmt.Errorf("invalid division %q: valid: %v", division, api.Divisions)
			}
			p := api.SearchParams{
				PageSize:      pageSize,
				Cities:        cities,
				Categories:    categories,
				Experiences:   experience,
				SearchTerms:   terms,
				MinimumSalary: minSalary,
			}
			raw, err := json.Marshal(p)
			if err != nil {
				return err
			}
			return withService(func(s *jobs.Service) error {
				w, err := s.Store.AddWatch(cmd.Context(), args[0], division, string(raw))
				if err != nil {
					return fail("watch add", err)
				}
				fmt.Printf("saved watch %q (division %s)\n", w.Name, w.Division)
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.StringVarP(&division, "division", "d", "IT", "division to search")
	f.StringSliceVar(&cities, "city", nil, "filter by city")
	f.StringSliceVar(&categories, "category", nil, "filter by category")
	f.StringSliceVar(&terms, "term", nil, "full-text term")
	f.StringSliceVar(&experience, "experience", nil, "filter by experience level")
	f.IntVar(&minSalary, "min-salary", 0, "minimum salary")
	f.IntVar(&pageSize, "page-size", 50, "results per sync poll")
	return cmd
}

func newWatchListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved watches",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				ws, err := s.Store.ListWatches(cmd.Context())
				if err != nil {
					return fail("watch list", err)
				}
				if gf.jsonOut {
					return printJSON(ws)
				}
				if len(ws) == 0 {
					fmt.Println("no watches")
					return nil
				}
				w := newTabWriter()
				fmt.Fprintln(w, "NAME\tDIVISION\tPARAMS")
				for _, x := range ws {
					fmt.Fprintf(w, "%s\t%s\t%s\n", x.Name, x.Division, x.Params)
				}
				w.Flush()
				return nil
			})
		},
	}
}

func newWatchRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Delete a watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				if err := s.Store.RemoveWatch(cmd.Context(), args[0]); err != nil {
					return fail("watch rm", err)
				}
				fmt.Printf("removed watch %q\n", args[0])
				return nil
			})
		},
	}
}
