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

func TestValidScopeKind(t *testing.T) {
	cases := map[string]bool{
		"division":         true,
		"mainCategory":     true,
		"subcategory":      true,
		"subcategoryGroup": true,
		"city":             true,
		"SubCategory":      true, // case-insensitive
		"nonsense":         false,
		"":                 false,
	}
	for in, want := range cases {
		if got := ValidScopeKind(in); got != want {
			t.Errorf("ValidScopeKind(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidMarketSection(t *testing.T) {
	cases := map[string]bool{
		"demand":       true,
		"salary":       true,
		"experience":   true,
		"topLocations": true,
		"topSkills":    true,
		"TopSkills":    true, // case-insensitive
		"salery":       false,
		"":             false,
	}
	for in, want := range cases {
		if got := ValidMarketSection(in); got != want {
			t.Errorf("ValidMarketSection(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBuildMarketStatsURL(t *testing.T) {
	c := NewClient("my-campaign")
	raw, err := c.buildMarketStatsURL("subcategory", "React", []string{"demand", "salary"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(u.Path, "/market-statistics/subcategory/React") {
		t.Errorf("path = %q, want suffix /market-statistics/subcategory/React", u.Path)
	}
	q := u.Query()
	if q.Get("campaign") != "my-campaign" {
		t.Errorf("campaign = %q", q.Get("campaign"))
	}
	if q.Get("fields") != "demand,salary" {
		t.Errorf("fields = %q, want demand,salary", q.Get("fields"))
	}
}

func TestBuildMarketStatsURLOmitsEmptyFields(t *testing.T) {
	c := NewClient("c")
	raw, _ := c.buildMarketStatsURL("city", "warszawa", nil)
	u, _ := url.Parse(raw)
	if u.Query().Has("fields") {
		t.Error("fields must be omitted when empty")
	}
}

func TestMarketStatisticsInvalidScopeKind(t *testing.T) {
	c := NewClient("test")
	if _, err := c.MarketStatistics(context.Background(), "bogus", "React", nil); err == nil {
		t.Error("want error for invalid scope kind")
	}
}

func TestMarketStatisticsDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Version") != apiVersion {
			t.Errorf("missing X-Api-Version header")
		}
		if got := r.URL.Query().Get("fields"); got != "demand,salary" {
			t.Errorf("fields = %q, want demand,salary", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"scopeKind":"subcategory","scopeKey":"React",
			"generatedAt":"2026-07-08T09:15:00Z",
			"includedSections":["demand","salary"],
			"demand":{"activeOffers":312,"distinctEmployers":148,"remoteOffers":121,
				"remotePercentage":39,"offerTrend":[{"period":"2026-Q1","offerCount":312}]},
			"salary":{"currency":"PLN","overall":{"min":8000,"p25":14000,"median":18000,
				"p75":23000,"max":38000},"b2b":{"median":20000,"average":20450,"offerCount":176},
				"permanent":null}}`))
	}))
	defer srv.Close()

	c := NewClient("test")
	c.BaseURL = srv.URL

	res, err := c.MarketStatistics(context.Background(), "subcategory", "React", []string{"demand", "salary"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ScopeKey != "React" || res.Demand == nil || res.Demand.ActiveOffers != 312 {
		t.Fatalf("unexpected demand: %+v", res.Demand)
	}
	if res.Salary == nil || res.Salary.Overall == nil || res.Salary.Overall.Median != 18000 {
		t.Fatalf("unexpected salary: %+v", res.Salary)
	}
	if res.Salary.B2B == nil || res.Salary.B2B.OfferCount != 176 {
		t.Fatalf("unexpected b2b: %+v", res.Salary.B2B)
	}
	if res.Salary.Permanent != nil {
		t.Errorf("permanent should be nil, got %+v", res.Salary.Permanent)
	}
	if len(res.Demand.OfferTrend) != 1 || res.Demand.OfferTrend[0].Period != "2026-Q1" {
		t.Errorf("unexpected trend: %+v", res.Demand.OfferTrend)
	}
}

// TestMarketStatisticsDecodesBuckets covers the []Bucket sections (experience,
// topLocations, topSkills) which TestMarketStatisticsDecodes never returns, so a
// wrong json tag on Bucket would otherwise ship undetected.
func TestMarketStatisticsDecodesBuckets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"scopeKind":"subcategory","scopeKey":"React",
			"generatedAt":"2026-07-08T09:15:00Z",
			"includedSections":["experience","topLocations","topSkills"],
			"experience":[{"label":"Senior","offerCount":120,"percentage":38}],
			"topLocations":[{"label":"Warszawa","offerCount":90,"percentage":29}],
			"topSkills":[{"label":"TypeScript","offerCount":210,"percentage":67}]}`))
	}))
	defer srv.Close()

	c := NewClient("test")
	c.BaseURL = srv.URL

	res, err := c.MarketStatistics(context.Background(), "subcategory", "React", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Experience) != 1 || res.Experience[0].Label != "Senior" ||
		res.Experience[0].OfferCount != 120 || res.Experience[0].Percentage != 38 {
		t.Errorf("unexpected experience: %+v", res.Experience)
	}
	if len(res.TopLocations) != 1 || res.TopLocations[0].Label != "Warszawa" ||
		res.TopLocations[0].OfferCount != 90 {
		t.Errorf("unexpected topLocations: %+v", res.TopLocations)
	}
	if len(res.TopSkills) != 1 || res.TopSkills[0].Label != "TypeScript" ||
		res.TopSkills[0].Percentage != 67 {
		t.Errorf("unexpected topSkills: %+v", res.TopSkills)
	}
}
