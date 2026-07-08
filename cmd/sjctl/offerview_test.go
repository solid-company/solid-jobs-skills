package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

func TestToLiteDropsHeavyFields(t *testing.T) {
	logo := "https://cdn.example/logo.png"
	from := 30000.0
	offers := []api.Offer{{
		JobOfferKey:     "k1",
		Title:           "Golang Developer",
		Company:         "Acme",
		Division:        "IT",
		Salary:          &api.Salary{From: &from, Currency: "PLN"},
		Locations:       []string{"Warszawa"},
		IsRemote:        true,
		ExperienceLevel: "Senior",
		Skills:          []api.NamedLevel{{Name: "Golang", Level: "Expert"}},
		CompanyLogoURL:  &logo,
		Benefits:        []string{"Pakiet medyczny"},
		Description:     "<div>SECRET_HEAVY_DESCRIPTION with lots of HTML</div>",
		ValidTo:         time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	}}

	lite := toLite(offers)
	if len(lite) != 1 {
		t.Fatalf("want 1 lite offer, got %d", len(lite))
	}

	raw, err := json.Marshal(lite)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	for _, banned := range []string{"SECRET_HEAVY_DESCRIPTION", "cdn.example", "Pakiet medyczny", "companyLogoUrl", "benefits", "description"} {
		if strings.Contains(got, banned) {
			t.Errorf("lite JSON should not contain %q, got: %s", banned, got)
		}
	}
	for _, want := range []string{"Golang Developer", "k1", "Senior", "2026-07-23"} {
		if !strings.Contains(got, want) {
			t.Errorf("lite JSON should contain %q, got: %s", want, got)
		}
	}
}

func TestHTMLToText(t *testing.T) {
	in := `<div class="well"><p><strong>Kogo szukamy?</strong></p><ul><li>Golang</li><li>SQL &amp; Oracle</li></ul><br>Koniec</div>`
	got := htmlToText(in)
	want := "Kogo szukamy?\n• Golang\n• SQL & Oracle\nKoniec"
	if got != want {
		t.Errorf("htmlToText mismatch:\n got: %q\nwant: %q", got, want)
	}

	if htmlToText("") != "" {
		t.Error("htmlToText(\"\") should be empty")
	}
}
