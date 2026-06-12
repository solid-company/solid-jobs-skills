package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
	"github.com/solid-company/solid-jobs-skills/internal/stats"
)

func newStatsCmd() *cobra.Command {
	var (
		division string
		skill    string
		city     string
		remote   bool
	)
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Salary and market stats over cached offers",
		Long: "Aggregate salary (min/median/max), experience-level counts and remote " +
			"share over offers already cached locally. Run searches first to populate the cache.",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := stats.Filter{Division: division, Skill: skill, City: city, Remote: remote}
			return withService(func(s *jobs.Service) error {
				res, err := s.SalaryStats(cmd.Context(), f)
				if err != nil {
					return fail("stats", err)
				}
				if gf.jsonOut {
					return printJSON(res)
				}
				if res.OfferCount == 0 {
					fmt.Println("no cached offers match (run a search first)")
					return nil
				}
				cur := res.Currency
				if cur == "" {
					cur = ""
				} else {
					cur = " " + cur
				}
				fmt.Printf("Offers:        %d (%d with salary)\n", res.OfferCount, res.WithSalary)
				if res.WithSalary > 0 {
					fmt.Printf("Salary min:    %d%s\n", res.SalaryMin, cur)
					fmt.Printf("Salary median: %d%s\n", res.SalaryMedian, cur)
					fmt.Printf("Salary max:    %d%s\n", res.SalaryMax, cur)
				}
				fmt.Printf("Remote share:  %.0f%% (%d offers)\n", res.RemoteShare*100, res.RemoteCount)
				if len(res.ByExperience) > 0 {
					fmt.Println("By experience:")
					for level, n := range res.ByExperience {
						fmt.Printf("  %-12s %d\n", level, n)
					}
				}
				return nil
			})
		},
	}
	f := cmd.Flags()
	f.StringVarP(&division, "division", "d", "", "limit to a division")
	f.StringVar(&skill, "skill", "", "match a skill or keyword")
	f.StringVar(&city, "city", "", "match a city")
	f.BoolVar(&remote, "remote", false, "only remote offers")
	return cmd
}
