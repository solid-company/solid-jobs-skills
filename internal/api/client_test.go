package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestValidCampaign(t *testing.T) {
	cases := map[string]bool{
		"solid-jobs-skills":     true,
		"abc123":                true,
		"a":                     true,
		"":                      false,
		"UPPER":                 false,
		"has space":             false,
		"under_score":           false,
		strings.Repeat("a", 64): true,
		strings.Repeat("a", 65): false,
	}
	for in, want := range cases {
		if got := ValidCampaign(in); got != want {
			t.Errorf("ValidCampaign(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidDivision(t *testing.T) {
	if !ValidDivision("IT") {
		t.Error("IT should be valid")
	}
	if ValidDivision("it") {
		t.Error("lowercase it should be invalid (case-sensitive)")
	}
	if ValidDivision("Nonsense") {
		t.Error("unknown division should be invalid")
	}
}

func TestBuildURL(t *testing.T) {
	c := NewClient("my-campaign")
	raw, err := c.buildURL("IT", SearchParams{
		PageIndex:     2,
		PageSize:      600, // clamped to 500
		SortActive:    "salaryFrom",
		SortDirection: "desc",
		Cities:        []string{"Poznań", "Warszawa"},
		Categories:    []string{"Developer"},
		Experiences:   []string{"Senior", "Regular"},
		SearchTerms:   []string{"golang"},
		MinimumSalary: 20000,
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()

	checks := map[string]string{
		"campaign":             "my-campaign",
		"pageIndex":            "2",
		"pageSize":             "500",
		"sortActive":           "salaryFrom",
		"sortDirection":        "desc",
		"search.cities":        "Poznań,Warszawa",
		"search.categories":    "Developer",
		"search.experiences":   "Senior,Regular",
		"search.searchTerm":    "golang",
		"search.minimumSalary": "20000",
	}
	for k, want := range checks {
		if got := q.Get(k); got != want {
			t.Errorf("query %q = %q, want %q", k, got, want)
		}
	}
	if !strings.HasSuffix(u.Path, "/offers/IT") {
		t.Errorf("path = %q, want suffix /offers/IT", u.Path)
	}
}

func TestBuildURLOmitsZeroValues(t *testing.T) {
	c := NewClient("c")
	raw, _ := c.buildURL("IT", SearchParams{})
	u, _ := url.Parse(raw)
	q := u.Query()
	for _, k := range []string{"pageIndex", "pageSize", "search.cities", "search.minimumSalary"} {
		if q.Has(k) {
			t.Errorf("expected %q to be omitted, got %q", k, q.Get(k))
		}
	}
	if q.Get("campaign") != "c" {
		t.Error("campaign must always be present")
	}
}

func TestSearchInvalidCampaign(t *testing.T) {
	c := NewClient("Bad Campaign")
	if _, err := c.Search(context.Background(), "IT", SearchParams{}); err != ErrInvalidCampaign {
		t.Errorf("want ErrInvalidCampaign, got %v", err)
	}
}

func TestSearchRetriesOn429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Version") != apiVersion {
			t.Errorf("missing X-Api-Version header")
		}
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"pageIndex":0,"pageSize":30,"totalCount":1,"totalPages":1,
			"jobs":[{"jobOfferKey":"k1","title":"Go Dev","company":"Acme",
			"salary":{"from":20000,"to":30000,"currency":"PLN"},"isRemote":true}]}`))
	}))
	defer srv.Close()

	c := NewClient("test")
	c.BaseURL = srv.URL
	c.MaxRetries = 3

	resp, err := c.Search(context.Background(), "IT", SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", calls)
	}
	if len(resp.Jobs) != 1 || resp.Jobs[0].JobOfferKey != "k1" {
		t.Fatalf("unexpected jobs: %+v", resp.Jobs)
	}
	if resp.Jobs[0].Salary == nil || *resp.Jobs[0].Salary.From != 20000 {
		t.Errorf("salary not decoded: %+v", resp.Jobs[0].Salary)
	}
	if !resp.Jobs[0].IsRemote {
		t.Error("isRemote should be true")
	}
}
