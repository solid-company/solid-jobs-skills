package main

import (
	"fmt"
	"io"
	"strings"

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
			"scopeKind: division, mainCategory, subcategory, subcategoryGroup, city, or the " +
			"special value `raport` for a single role's 3-year yearly trend report (no --fields).\n" +
			"scopeKey:  a value within the kind, e.g. IT, Developer, React, Frontend, warszawa, " +
			"or a role name (e.g. Golang) when scopeKind is `raport`.\n\n" +
			"Examples:\n" +
			"  sjctl market subcategory React\n" +
			"  sjctl market division IT --fields demand,salary\n" +
			"  sjctl market city warszawa --json\n" +
			"  sjctl market raport Golang --json",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeKind, scopeKey := args[0], args[1]

			if api.IsRaportScopeKind(scopeKind) {
				if len(fields) > 0 {
					return fmt.Errorf("--fields is not supported for market raport")
				}
				return withService(func(s *jobs.Service) error {
					res, err := s.MarketRaport(cmd.Context(), scopeKey)
					if err != nil {
						return fail("market", err)
					}
					if gf.jsonOut {
						return printJSON(res)
					}
					printMarketRaport(cmd.OutOrStdout(), res)
					return nil
				})
			}

			if !api.ValidScopeKind(scopeKind) {
				return fmt.Errorf("invalid scope kind %q: valid: %v (or %q)", scopeKind, api.ScopeKinds, api.ScopeKindRaport)
			}
			for _, f := range fields {
				if !api.ValidMarketSection(f) {
					return fmt.Errorf("invalid --fields section %q: valid: %v", f, api.MarketSections)
				}
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
		"limit to sections (comma-separated): "+strings.Join(api.MarketSections, ", "))
	return cmd
}

// printMarketStats renders a MarketStats response as a human-readable report.
func printMarketStats(m *api.MarketStats) {
	fmt.Printf("Scope:    %s / %s\n", m.ScopeKind, m.ScopeKey)
	fmt.Printf("As of:    %s\n", m.GeneratedAt.Format("2006-01-02 15:04 MST"))
	fmt.Printf("Sections: %v\n", m.IncludedSections)

	if m.Demand == nil && m.Salary == nil &&
		len(m.Experience) == 0 && len(m.TopLocations) == 0 && len(m.TopSkills) == 0 {
		fmt.Println("\nno market data for this scope")
		return
	}

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
		fmt.Printf("  %-10s (no precomputed data)\n", label+":")
		return
	}
	fmt.Printf("  %-10s median %.0f | average %.0f | %d offers\n", label+":", s.Median, s.Average, s.OfferCount)
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

// raportTopSkillsShown caps how many per-year topSkills entries the human
// renderer prints; --json carries the full list.
const raportTopSkillsShown = 10

// printMarketRaport renders a MarketRaport response as a human-readable
// year-by-year report.
func printMarketRaport(w io.Writer, r *api.MarketRaport) {
	fmt.Fprintf(w, "Role:         %s\n", r.ScopeKey)
	fmt.Fprintf(w, "Generated at: %s\n", r.GeneratedAt.Format("2006-01-02 15:04 MST"))
	fmt.Fprintf(w, "Years:        %d\n", len(r.Years))

	if len(r.Years) == 0 {
		fmt.Fprintln(w, "\nno raport data for this role")
		return
	}

	for _, year := range r.Years {
		printRaportYear(w, year)
	}
}

func printRaportYear(w io.Writer, year api.RaportYear) {
	fmt.Fprintf(w, "\n[%d]  offerCount=%d\n", year.Year, year.OfferCount)

	c := year.ContractType
	fmt.Fprintf(w, "  contractType (total=%d):\n", c.Total)
	printRaportBucket(w, "b2bOnly", c.B2BOnly, c.Total)
	printRaportBucket(w, "permanentOnly", c.PermanentOnly, c.Total)
	printRaportBucket(w, "both", c.Both, c.Total)

	s := year.Seniority
	fmt.Fprintf(w, "  seniority (total=%d):\n", s.Total)
	printRaportSeniorityNode(w, "junior", s.Junior)
	printRaportSeniorityNode(w, "regular", s.Regular)
	printRaportSeniorityNode(w, "senior", s.Senior)

	fmt.Fprintln(w, "  salary (assumed PLN/month, per seniority — lower..upper band):")
	printRaportSalary(w, "B2B", year.SalaryB2B)
	printRaportSalary(w, "Permanent", year.SalaryPermanent)

	fmt.Fprintf(w, "  topSkills (%d):\n", len(year.TopSkills))
	shown := len(year.TopSkills)
	if shown > raportTopSkillsShown {
		shown = raportTopSkillsShown
	}
	for _, skill := range year.TopSkills[:shown] {
		fmt.Fprintf(w, "    %-20s %d offers\n", skill.Name, skill.Count)
	}
	if rest := len(year.TopSkills) - shown; rest > 0 {
		fmt.Fprintf(w, "    ... and %d more (use --json)\n", rest)
	}
}

func printRaportBucket(w io.Writer, label string, bucket api.CountWithPercentage, total int) {
	fmt.Fprintf(w, "    %-16s %5d offers  (%d%% of %d)\n", label, bucket.Count, bucket.Percentage, total)
}

func printRaportSeniorityNode(w io.Writer, label string, node api.SeniorityNode) {
	fmt.Fprintf(w, "    %-16s %5d offers  (%d%% of seniority.total)\n", label, node.Count, node.Percentage)
	c := node.ContractType
	printRaportBucket(w, "  b2bOnly", c.B2BOnly, c.Total)
	printRaportBucket(w, "  permanentOnly", c.PermanentOnly, c.Total)
	printRaportBucket(w, "  both", c.Both, c.Total)
}

func printRaportSalaryBand(w io.Writer, label string, band api.RaportSalaryBand) {
	if band.SalaryRangeCount == 0 {
		fmt.Fprintf(w, "    %-14s (no salary data)\n", label)
		return
	}
	fmt.Fprintf(w, "    %-14s median=%.0f..%.0f  average=%.0f..%.0f  ranges=%d\n",
		label, band.MedianLower, band.MedianUpper, band.AverageLower, band.AverageUpper, band.SalaryRangeCount)
}

func printRaportSalary(w io.Writer, label string, salary *api.RaportSalaryStat) {
	if salary == nil {
		fmt.Fprintf(w, "    %s: (no data for this year)\n", label)
		return
	}
	printRaportSalaryBand(w, label+" junior", salary.Junior)
	printRaportSalaryBand(w, label+" regular", salary.Regular)
	printRaportSalaryBand(w, label+" senior", salary.Senior)
}
