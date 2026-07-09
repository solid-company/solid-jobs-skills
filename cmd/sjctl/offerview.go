package main

import (
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

// offerLite is the token-lean projection of an Offer used for list output
// (search, sync). It keeps validTo (needed for expiry) but drops the heavy
// fields — the raw HTML description, logo URL, benefits, secondary salary and
// the create/publish timestamps — that browse/triage never reads. The full
// offer is still cached in SQLite by UpsertOffers and retrievable in full via
// `sjctl offer show <key>`.
type offerLite struct {
	JobOfferKey     string           `json:"jobOfferKey"`
	Title           string           `json:"title"`
	Company         string           `json:"company"`
	Division        string           `json:"division"`
	Category        string           `json:"category"`
	SubCategory     string           `json:"subCategory"`
	Salary          *api.Salary      `json:"salary"`
	ContractTime    string           `json:"contractTime"`
	Locations       []string         `json:"locations"`
	IsRemote        bool             `json:"isRemote"`
	IsHybrid        bool             `json:"isHybrid"`
	URL             string           `json:"url"`
	ExperienceLevel string           `json:"experienceLevel"`
	Skills          []api.NamedLevel `json:"skills"`
	Languages       []api.NamedLevel `json:"languages"`
	ValidTo         string           `json:"validTo,omitempty"`
}

// toLiteOne projects a single offer into its lean form.
func toLiteOne(o api.Offer) offerLite {
	return offerLite{
		JobOfferKey:     o.JobOfferKey,
		Title:           o.Title,
		Company:         o.Company,
		Division:        o.Division,
		Category:        o.Category,
		SubCategory:     o.SubCategory,
		Salary:          o.Salary,
		ContractTime:    o.ContractTime,
		Locations:       o.Locations,
		IsRemote:        o.IsRemote,
		IsHybrid:        o.IsHybrid,
		URL:             o.URL,
		ExperienceLevel: o.ExperienceLevel,
		Skills:          o.Skills,
		Languages:       o.Languages,
		ValidTo:         fmtDate(o.ValidTo),
	}
}

// toLite projects offers into their lean form for list output.
func toLite(offers []api.Offer) []offerLite {
	out := make([]offerLite, len(offers))
	for i := range offers {
		out[i] = toLiteOne(offers[i])
	}
	return out
}

// fmtDate renders a validity timestamp as an ISO date, or "" when zero.
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

var (
	htmlTagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// liOpenRe matches an opening <li>, including attributed forms like
	// <li class="…"> that real API HTML carries, so each list item keeps its
	// bullet rather than losing it to the generic tag strip.
	liOpenRe = regexp.MustCompile(`(?i)<li\b[^>]*>`)
)

// htmlToText converts an offer's HTML description into compact plain text:
// block tags become line breaks, list items get a bullet, remaining tags are
// stripped, entities decoded, and blank lines collapsed. The raw HTML stays in
// the database; this is applied only on output to save tokens and read cleanly.
func htmlToText(s string) string {
	if s == "" {
		return ""
	}
	s = liOpenRe.ReplaceAllString(s, "\n• ")
	repl := strings.NewReplacer(
		"</li>", "\n", "</p>", "\n", "</ul>", "\n", "</div>", "\n",
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
	)
	s = repl.Replace(s)
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if ln = strings.TrimSpace(ln); ln != "" {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}
