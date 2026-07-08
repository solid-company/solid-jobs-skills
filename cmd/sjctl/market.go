package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/solid-company/solid-jobs-skills/internal/api"
	"github.com/solid-company/solid-jobs-skills/internal/jobs"
)

func newMarketCmd() *cobra.Command {
	var fields []string

	cmd := &cobra.Command{
		Use:   "market <scopeKind> <scopeKey>",
		Short: "Live market statistics for a scope from the public API",
		Long: "Fetch server-side market statistics for a single scope directly from the " +
			"SOLID.Jobs API — demand, salary bands, experience mix and top locations/skills " +
			"for a whole division, category, specialization, subcategory group or city.\n\n" +
			"Unlike `stats` (which aggregates offers already cached locally), `market` reflects " +
			"the entire live market and needs no prior search.\n\n" +
			"scopeKind: division, mainCategory, subcategory, subcategoryGroup, city.\n" +
			"scopeKey:  a value within the kind, e.g. IT, Developer, React, Frontend, warszawa.\n\n" +
			"Examples:\n" +
			"  sjctl market subcategory React\n" +
			"  sjctl market division IT --fields demand,salary\n" +
			"  sjctl market city warszawa --json",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeKind, scopeKey := args[0], args[1]
			if !api.ValidScopeKind(scopeKind) {
				return fmt.Errorf("invalid scope kind %q: valid: %v", scopeKind, api.ScopeKinds)
			}
			return withService(func(s *jobs.Service) error {
				res, err := s.MarketStatistics(cmd.Context(), scopeKind, scopeKey, fields)
				if err != nil {
					return fail("market", err)
				}
				if gf.jsonOut {
					return printJSON(res)
				}
				printMarketStats(res)
				return nil
			})
		},
	}
	cmd.Flags().StringSliceVar(&fields, "fields", nil,
		"limit to sections (comma-separated): demand, salary, experience, topLocations, topSkills")
	return cmd
}

// printMarketStats renders a MarketStats response as a human-readable report.
func printMarketStats(m *api.MarketStats) {
	fmt.Printf("Scope:    %s / %s\n", m.ScopeKind, m.ScopeKey)
	fmt.Printf("As of:    %s\n", m.GeneratedAt.Format("2006-01-02 15:04 MST"))
	fmt.Printf("Sections: %v\n", m.IncludedSections)

	if d := m.Demand; d != nil {
		fmt.Println("\nDemand:")
		fmt.Printf("  Active offers:      %d\n", d.ActiveOffers)
		fmt.Printf("  Distinct employers: %d\n", d.DistinctEmployers)
		fmt.Printf("  Remote:             %d (%d%%)\n", d.RemoteOffers, d.RemotePercentage)
		if len(d.OfferTrend) > 0 {
			fmt.Println("  Offer trend (quarterly):")
			for _, p := range d.OfferTrend {
				fmt.Printf("    %-8s %d\n", p.Period, p.OfferCount)
			}
		}
	}

	if s := m.Salary; s != nil {
		fmt.Printf("\nSalary (%s):\n", s.Currency)
		if b := s.Overall; b != nil {
			fmt.Printf("  Overall:   min %.0f | p25 %.0f | median %.0f | p75 %.0f | max %.0f\n",
				b.Min, b.P25, b.Median, b.P75, b.Max)
		} else {
			fmt.Println("  Overall:   (no offers with a declared salary)")
		}
		printContractSalary("B2B", s.B2B)
		printContractSalary("Permanent", s.Permanent)
	}

	printBucketSection("Experience", m.Experience)
	printBucketSection("Top locations", m.TopLocations)
	printBucketSection("Top skills", m.TopSkills)
}

func printContractSalary(label string, s *api.SalaryStat) {
	if s == nil {
		fmt.Printf("  %-9s (no precomputed data)\n", label+":")
		return
	}
	fmt.Printf("  %-9s median %.0f | average %.0f | %d offers\n", label+":", s.Median, s.Average, s.OfferCount)
}

func printBucketSection(title string, buckets []api.Bucket) {
	if len(buckets) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	w := newTabWriter()
	for _, b := range buckets {
		fmt.Fprintf(w, "  %s\t%d offers\t%d%%\n", b.Label, b.OfferCount, b.Percentage)
	}
	w.Flush()
}
