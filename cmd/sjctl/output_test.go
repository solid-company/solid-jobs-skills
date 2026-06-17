package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

func TestPrintOffersTableIncludesLink(t *testing.T) {
	offers := []api.Offer{
		{
			JobOfferKey: "abc123",
			Title:       "Senior Go Developer",
			Company:     "Acme",
			IsRemote:    true,
			URL:         "https://solid.jobs/offers/abc123",
		},
	}

	out := captureStdout(t, func() { printOffersTable(offers) })

	if !strings.Contains(out, "LINK") {
		t.Errorf("header missing LINK column:\n%s", out)
	}
	if !strings.Contains(out, "https://solid.jobs/offers/abc123") {
		t.Errorf("row missing offer URL:\n%s", out)
	}
	// LINK must be the last column: header order and row order agree.
	header := strings.SplitN(out, "\n", 2)[0]
	if !strings.HasSuffix(strings.TrimRight(header, " "), "LINK") {
		t.Errorf("LINK is not the last header column: %q", header)
	}
	// URL printed bare (no OSC-8 hyperlink escapes) so tabwriter alignment holds.
	if strings.Contains(out, "\x1b]8;") {
		t.Errorf("URL should not be wrapped in OSC-8 escapes:\n%q", out)
	}
}

func TestPrintOffersTableEmptyURL(t *testing.T) {
	offers := []api.Offer{
		{JobOfferKey: "nourl", Title: "Job", Company: "Co"},
	}

	out := captureStdout(t, func() { printOffersTable(offers) })

	// A missing URL must not break the row; the key still renders.
	if !strings.Contains(out, "nourl") {
		t.Errorf("row missing for offer with empty URL:\n%s", out)
	}
}
