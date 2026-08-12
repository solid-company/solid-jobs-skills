package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/solid-company/solid-jobs-skills/internal/api"
)

func TestPrintMarketRaportEmptyYears(t *testing.T) {
	var buf bytes.Buffer
	printMarketRaport(&buf, &api.MarketRaport{ScopeKey: "Nonexistent"})
	out := buf.String()
	if !strings.Contains(out, "no raport data for this role") {
		t.Errorf("expected an explicit no-data message, got:\n%s", out)
	}
}

func TestPrintMarketRaportZeroSalaryBandNotShownAsMoney(t *testing.T) {
	var buf bytes.Buffer
	printMarketRaport(&buf, &api.MarketRaport{
		ScopeKey: "Golang",
		Years: []api.RaportYear{{
			Year:       2023,
			OfferCount: 20,
			SalaryB2B: &api.RaportSalaryStat{
				// Junior has SalaryRangeCount 0 — no data, not PLN 0.
				Junior:  api.RaportSalaryBand{SalaryRangeCount: 0},
				Regular: api.RaportSalaryBand{MedianLower: 22250, MedianUpper: 29500, AverageLower: 23290, AverageUpper: 30430, SalaryRangeCount: 10},
			},
		}},
	})
	out := buf.String()
	if strings.Contains(out, "median=0..0") {
		t.Errorf("zero salary band must not be printed as a real 0 PLN figure, got:\n%s", out)
	}
	if !strings.Contains(out, "B2B junior     (no salary data)") {
		t.Errorf("expected an explicit no-data line for the zero band, got:\n%s", out)
	}
	if !strings.Contains(out, "median=22250..29500") {
		t.Errorf("expected the real regular band to still print, got:\n%s", out)
	}
}

func TestPrintMarketRaportTruncatesTopSkillsWithMarker(t *testing.T) {
	skills := make([]api.RaportSkill, 15)
	for i := range skills {
		skills[i] = api.RaportSkill{Name: "skill", Count: 15 - i}
	}
	var buf bytes.Buffer
	printMarketRaport(&buf, &api.MarketRaport{
		ScopeKey: "Golang",
		Years:    []api.RaportYear{{Year: 2026, TopSkills: skills}},
	})
	out := buf.String()
	if strings.Count(out, "skill") != raportTopSkillsShown {
		t.Errorf("expected exactly %d skill lines, got %d in:\n%s", raportTopSkillsShown, strings.Count(out, "skill"), out)
	}
	if !strings.Contains(out, "... and 5 more (use --json)") {
		t.Errorf("expected a truncation marker for the remaining 5 skills, got:\n%s", out)
	}
}

func TestPrintMarketRaportUnderTenSkillsNoMarker(t *testing.T) {
	var buf bytes.Buffer
	printMarketRaport(&buf, &api.MarketRaport{
		ScopeKey: "Golang",
		Years:    []api.RaportYear{{Year: 2026, TopSkills: []api.RaportSkill{{Name: "Go", Count: 5}}}},
	})
	out := buf.String()
	if strings.Contains(out, "more (use --json)") {
		t.Errorf("no truncation marker expected under the cap, got:\n%s", out)
	}
}

func TestPrintMarketRaportRendersQuarters(t *testing.T) {
	var buf bytes.Buffer
	printMarketRaport(&buf, &api.MarketRaport{
		ScopeKey: "Golang",
		Years: []api.RaportYear{{
			Year:       2026,
			OfferCount: 20,
			Quarters: []api.RaportQuarter{
				{Quarter: 1, OfferCount: 11},
				{Quarter: 2, OfferCount: 9},
			},
		}},
	})
	out := buf.String()
	if !strings.Contains(out, "quarters (2):") {
		t.Errorf("expected a quarters count header, got:\n%s", out)
	}
	if !strings.Contains(out, "Q1  offerCount=11") {
		t.Errorf("expected Q1 offerCount line, got:\n%s", out)
	}
	if !strings.Contains(out, "Q2  offerCount=9") {
		t.Errorf("expected Q2 offerCount line, got:\n%s", out)
	}
}

func TestPrintMarketRaportNoQuartersOmitsSection(t *testing.T) {
	var buf bytes.Buffer
	printMarketRaport(&buf, &api.MarketRaport{
		ScopeKey: "Golang",
		Years:    []api.RaportYear{{Year: 2026, OfferCount: 20}},
	})
	out := buf.String()
	if strings.Contains(out, "quarters (") {
		t.Errorf("no quarters section expected when Quarters is empty, got:\n%s", out)
	}
}
