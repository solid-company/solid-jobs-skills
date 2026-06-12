package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
)

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Run saved watches and report new offers",
		Long: "Execute every saved watch, cache results, and print offers not seen " +
			"before. A second run reports nothing until new offers appear.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withService(func(s *jobs.Service) error {
				res, err := s.RunSync(cmd.Context())
				if err != nil {
					return fail("sync", err)
				}
				if gf.jsonOut {
					return printJSON(res)
				}
				if res.WatchesRun == 0 {
					fmt.Println("no watches configured (add one with: sjctl watch add)")
					return nil
				}
				if len(res.New) == 0 {
					fmt.Printf("%d watches run, %d offers checked, no new offers\n",
						res.WatchesRun, res.TotalSeen)
					return nil
				}
				w := newTabWriter()
				fmt.Fprintln(w, "WATCH\tKEY\tTITLE\tCOMPANY\tSALARY")
				for _, n := range res.New {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						n.Watch, n.Offer.JobOfferKey,
						truncate(n.Offer.Title, 36), truncate(n.Offer.Company, 22),
						salaryStr(n.Offer.Salary))
				}
				w.Flush()
				fmt.Printf("\n%d new offers across %d watches\n", len(res.New), res.WatchesRun)
				return nil
			})
		},
	}
}
