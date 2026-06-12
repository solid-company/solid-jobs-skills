package stats

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/solid-company/solid-jobs-skills/internal/api"
	"github.com/solid-company/solid-jobs-skills/internal/store"
)

// openStore opens a throwaway SQLite store seeded with the given offers.
func openStore(t *testing.T, offers ...api.Offer) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if len(offers) > 0 {
		if err := st.UpsertOffers(context.Background(), offers); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	return st
}

func salaryOffer(key, division, currency string, from, to float64, remote bool, level string) api.Offer {
	return api.Offer{
		JobOfferKey:     key,
		Title:           "Go Developer",
		Company:         "Acme",
		Division:        division,
		ExperienceLevel: level,
		Salary:          &api.Salary{From: &from, To: &to, Currency: currency},
		IsRemote:        remote,
	}
}

func TestComputeSalaryAggregates(t *testing.T) {
	st := openStore(t,
		salaryOffer("a", "IT", "PLN", 10000, 20000, true, "Senior"),  // midpoint 15000
		salaryOffer("b", "IT", "PLN", 20000, 30000, false, "Mid"),    // midpoint 25000
		salaryOffer("c", "IT", "PLN", 30000, 40000, true, "Senior"),  // midpoint 35000
	)

	res, err := Compute(context.Background(), st.Queries(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.OfferCount != 3 || res.WithSalary != 3 {
		t.Fatalf("counts: offers=%d withSalary=%d", res.OfferCount, res.WithSalary)
	}
	if res.SalaryMin != 15000 || res.SalaryMedian != 25000 || res.SalaryMax != 35000 {
		t.Errorf("salary min/median/max = %d/%d/%d, want 15000/25000/35000",
			res.SalaryMin, res.SalaryMedian, res.SalaryMax)
	}
	if res.Currency != "PLN" {
		t.Errorf("currency = %q, want PLN", res.Currency)
	}
	if res.RemoteCount != 2 || res.RemoteShare != 2.0/3.0 {
		t.Errorf("remote count=%d share=%v", res.RemoteCount, res.RemoteShare)
	}
	if res.ByExperience["Senior"] != 2 || res.ByDivision["IT"] != 3 {
		t.Errorf("breakdowns: %+v / %+v", res.ByExperience, res.ByDivision)
	}
}

// TestComputeDominantCurrency pins the documented single-currency assumption:
// the reported Currency is the majority vote, and salaries are pooled across
// currencies (so a mixed cache produces a mislabelled blend — known caveat).
func TestComputeDominantCurrency(t *testing.T) {
	st := openStore(t,
		salaryOffer("a", "IT", "PLN", 15000, 15000, true, "Senior"),
		salaryOffer("b", "IT", "PLN", 25000, 25000, true, "Senior"),
		salaryOffer("c", "IT", "EUR", 3500, 3500, true, "Senior"),
	)
	res, err := Compute(context.Background(), st.Queries(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Currency != "PLN" {
		t.Errorf("currency = %q, want PLN (majority)", res.Currency)
	}
	// Pooled min straddles currencies — documents the caveat rather than hiding it.
	if res.SalaryMin != 3500 {
		t.Errorf("pooled min = %d, want 3500 (EUR value mixed in)", res.SalaryMin)
	}
}

func TestComputeSkillCityFilters(t *testing.T) {
	golang := salaryOffer("go", "IT", "PLN", 20000, 20000, true, "Senior")
	golang.Skills = []api.NamedLevel{{Name: "Golang", Level: "advanced"}}
	golang.Locations = []string{"Warszawa"}

	rust := salaryOffer("rs", "IT", "PLN", 22000, 22000, true, "Senior")
	rust.Skills = []api.NamedLevel{{Name: "Rust", Level: "advanced"}}
	rust.Locations = []string{"Kraków"}

	st := openStore(t, golang, rust)

	bySkill, err := Compute(context.Background(), st.Queries(), Filter{Skill: "Golang"})
	if err != nil {
		t.Fatal(err)
	}
	if bySkill.OfferCount != 1 {
		t.Errorf("skill filter matched %d offers, want 1", bySkill.OfferCount)
	}

	byCity, err := Compute(context.Background(), st.Queries(), Filter{City: "Kraków"})
	if err != nil {
		t.Fatal(err)
	}
	if byCity.OfferCount != 1 {
		t.Errorf("city filter matched %d offers, want 1", byCity.OfferCount)
	}
}
