package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/jobs"
)

// newOfferLite is the token-lean shape of a newly-seen offer in `sync --json`.
type newOfferLite struct {
	Watch string    `json:"watch"`
	Offer offerLite `json:"offer"`
}

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
					// Lean projection: drop the heavy per-offer fields (HTML
					// description, logo, benefits) the digest triage never reads.
					// Full offers stay cached and are retrievable via `offer show`.
					newLite := make([]newOfferLite, len(res.New))
					for i, n := range res.New {
						newLite[i] = newOfferLite{Watch: n.Watch, Offer: toLiteOne(n.Offer)}
					}
					return printJSON(map[string]any{
						"watchesRun": res.WatchesRun,
						"totalSeen":  res.TotalSeen,
						"new":        newLite,
					})
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
