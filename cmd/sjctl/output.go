package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

// newTabWriter returns a tabwriter writing to stdout.
func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

// salaryStr renders a salary band compactly, e.g. "15000-22000 PLN".
func salaryStr(s *api.Salary) string {
	if s == nil {
		return "-"
	}
	var band string
	switch {
	case s.From != nil && s.To != nil:
		band = fmt.Sprintf("%.0f-%.0f", *s.From, *s.To)
	case s.From != nil:
		band = fmt.Sprintf("%.0f+", *s.From)
	case s.To != nil:
		band = fmt.Sprintf("<%.0f", *s.To)
	default:
		return "-"
	}
	if s.Currency != "" {
		band += " " + s.Currency
	}
	return band
}

// workMode summarises remote/hybrid flags.
func workMode(o *api.Offer) string {
	switch {
	case o.IsRemote:
		return "remote"
	case o.IsHybrid:
		return "hybrid"
	default:
		return "onsite"
	}
}

// truncate shortens s to n runes with an ellipsis.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// printOffersTable renders offers as a table.
func printOffersTable(offers []api.Offer) {
	w := newTabWriter()
	fmt.Fprintln(w, "KEY\tTITLE\tCOMPANY\tSALARY\tMODE\tLOCATION\tLINK")
	for i := range offers {
		o := &offers[i]
		loc := strings.Join(o.Locations, ", ")
		if o.IsRemote {
			loc = "Remote"
		}
		// LINK is printed as a bare URL (last column) so it stays clickable in
		// modern terminals; it is not wrapped in OSC-8 escapes because tabwriter
		// counts escape bytes as width and would misalign the columns.
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			o.JobOfferKey,
			truncate(o.Title, 40),
			truncate(o.Company, 24),
			salaryStr(o.Salary),
			workMode(o),
			truncate(loc, 24),
			o.URL,
		)
	}
	w.Flush()
}
