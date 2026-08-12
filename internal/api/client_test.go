package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestBuildMarketRaportURL(t *testing.T) {
	c := NewClient("my-campaign")
	raw, err := c.buildMarketRaportURL("Golang")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(u.Path, "/market-statistics/raport/Golang") {
		t.Errorf("path = %q, want suffix /market-statistics/raport/Golang", u.Path)
	}
	q := u.Query()
	if q.Get("campaign") != "my-campaign" {
		t.Errorf("campaign = %q", q.Get("campaign"))
	}
	if q.Has("fields") {
		t.Error("raport must never send a fields param")
	}
}

func TestBuildMarketRaportURLEscapesRole(t *testing.T) {
	c := NewClient("my-campaign")
	// "C#" contains a character (#) that starts a URL fragment if not escaped;
	// an unescaped path silently truncates to ".../raport/C" and would fetch a
	// different role's report instead of erroring.
	raw, err := c.buildMarketRaportURL("C#")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(u.Path, "/market-statistics/raport/C%23") &&
		!strings.HasSuffix(u.EscapedPath(), "/market-statistics/raport/C%23") {
		t.Errorf("escaped path = %q, want suffix .../raport/C%%23", u.EscapedPath())
	}
	if u.Fragment != "" {
		t.Errorf("fragment = %q, want empty — # must be escaped, not treated as a fragment marker", u.Fragment)
	}
	last := u.Path[strings.LastIndex(u.Path, "/")+1:]
	if last != "C#" {
		t.Errorf("decoded last path segment = %q, want %q", last, "C#")
	}
}

func TestMarketRaportEmptyScopeKey(t *testing.T) {
	c := NewClient("test")
	if _, err := c.MarketRaport(context.Background(), ""); err == nil {
		t.Error("want error for empty scope key")
	}
}

func TestMarketRaportInvalidCampaign(t *testing.T) {
	c := NewClient("Bad Campaign")
	if _, err := c.MarketRaport(context.Background(), "Golang"); err != ErrInvalidCampaign {
		t.Errorf("want ErrInvalidCampaign, got %v", err)
	}
}

// TestMarketRaportDecodes fixture is a trimmed, byte-for-byte-faithful capture
// of the live GET .../market-statistics/raport/Golang response (2026-08-07).
// The 2023 year's seniority nodes are captured as-is even though they look
// inconsistent with the model doc comments: "regular"/"senior" have
// count:0/percentage:0 at the SeniorityNode level while their nested
// ContractType breakdown is fully populated (10 offers each). That is a real
// quirk of the live API, not a fixture bug — don't "fix" it into consistency.
func TestMarketRaportDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Version") != apiVersion {
			t.Errorf("missing X-Api-Version header")
		}
		if r.URL.Query().Has("fields") {
			t.Error("raport request must not send fields")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"scopeKey":"Golang",
			"generatedAt":"2026-08-07T07:46:10.5024306+00:00",
			"years":[
				{
					"year":2023,"offerCount":20,
					"contractType":{"b2bOnly":{"count":0,"percentage":0},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":0},
					"seniority":{
						"junior":{"count":0,"percentage":0,"contractType":{"b2bOnly":{"count":0,"percentage":0},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":0}},
						"regular":{"count":0,"percentage":0,"contractType":{"b2bOnly":{"count":10,"percentage":100},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":10}},
						"senior":{"count":0,"percentage":0,"contractType":{"b2bOnly":{"count":10,"percentage":100},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":10}},
						"total":0},
					"salaryB2B":{
						"junior":{"medianLower":0,"medianUpper":0,"averageLower":0,"averageUpper":0,"salaryRangeCount":0},
						"regular":{"medianLower":22250,"medianUpper":29500,"averageLower":23290,"averageUpper":30430,"salaryRangeCount":10},
						"senior":{"medianLower":24350,"medianUpper":30350,"averageLower":22450,"averageUpper":29640,"salaryRangeCount":10}},
					"salaryUoP":null,
					"topSkills":[{"name":"Golang","count":20},{"name":"Kubernetes","count":9}]
				},
				{
					"year":2024,"offerCount":43,
					"contractType":{"b2bOnly":{"count":38,"percentage":88},"permanentOnly":{"count":4,"percentage":9},"both":{"count":1,"percentage":2},"total":43},
					"seniority":{
						"junior":{"count":0,"percentage":0,"contractType":{"b2bOnly":{"count":0,"percentage":0},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":0}},
						"regular":{"count":22,"percentage":51,"contractType":{"b2bOnly":{"count":20,"percentage":91},"permanentOnly":{"count":2,"percentage":9},"both":{"count":0,"percentage":0},"total":22}},
						"senior":{"count":21,"percentage":49,"contractType":{"b2bOnly":{"count":18,"percentage":86},"permanentOnly":{"count":2,"percentage":10},"both":{"count":1,"percentage":5},"total":21}},
						"total":43},
					"salaryB2B":{
						"junior":{"medianLower":0,"medianUpper":0,"averageLower":0,"averageUpper":0,"salaryRangeCount":0},
						"regular":{"medianLower":21800,"medianUpper":26000,"averageLower":20930,"averageUpper":26325,"salaryRangeCount":20},
						"senior":{"medianLower":23500,"medianUpper":26900,"averageLower":23068,"averageUpper":27847,"salaryRangeCount":19}},
					"salaryUoP":{
						"junior":{"medianLower":0,"medianUpper":0,"averageLower":0,"averageUpper":0,"salaryRangeCount":0},
						"regular":{"medianLower":14500,"medianUpper":18500,"averageLower":14500,"averageUpper":18500,"salaryRangeCount":2},
						"senior":{"medianLower":18000,"medianUpper":22000,"averageLower":19000,"averageUpper":23333,"salaryRangeCount":3}},
					"topSkills":[{"name":"Golang","count":43},{"name":"Kubernetes","count":19}],
					"quarters":[
						{
							"quarter":1,"offerCount":11,
							"contractType":{"b2bOnly":{"count":9,"percentage":82},"permanentOnly":{"count":2,"percentage":18},"both":{"count":0,"percentage":0},"total":11},
							"seniority":{
								"junior":{"count":0,"percentage":0,"contractType":{"b2bOnly":{"count":0,"percentage":0},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":0}},
								"regular":{"count":6,"percentage":55,"contractType":{"b2bOnly":{"count":5,"percentage":83},"permanentOnly":{"count":1,"percentage":17},"both":{"count":0,"percentage":0},"total":6}},
								"senior":{"count":5,"percentage":45,"contractType":{"b2bOnly":{"count":4,"percentage":80},"permanentOnly":{"count":1,"percentage":20},"both":{"count":0,"percentage":0},"total":5}},
								"total":11},
							"salaryB2B":{
								"junior":{"medianLower":0,"medianUpper":0,"averageLower":0,"averageUpper":0,"salaryRangeCount":0},
								"regular":{"medianLower":21000,"medianUpper":25000,"averageLower":20500,"averageUpper":25500,"salaryRangeCount":5},
								"senior":{"medianLower":23000,"medianUpper":27000,"averageLower":22500,"averageUpper":27500,"salaryRangeCount":4}}
						},
						{
							"quarter":2,"offerCount":9,
							"contractType":{"b2bOnly":{"count":9,"percentage":100},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":9},
							"seniority":{
								"junior":{"count":0,"percentage":0,"contractType":{"b2bOnly":{"count":0,"percentage":0},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":0}},
								"regular":{"count":4,"percentage":44,"contractType":{"b2bOnly":{"count":4,"percentage":100},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":4}},
								"senior":{"count":5,"percentage":56,"contractType":{"b2bOnly":{"count":5,"percentage":100},"permanentOnly":{"count":0,"percentage":0},"both":{"count":0,"percentage":0},"total":5}},
								"total":9},
							"salaryB2B":{
								"junior":{"medianLower":0,"medianUpper":0,"averageLower":0,"averageUpper":0,"salaryRangeCount":0},
								"regular":{"medianLower":22000,"medianUpper":26000,"averageLower":21500,"averageUpper":26500,"salaryRangeCount":4},
								"senior":{"medianLower":24000,"medianUpper":28000,"averageLower":23500,"averageUpper":28500,"salaryRangeCount":5}}
						}
					]
				}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient("test")
	c.BaseURL = srv.URL

	res, err := c.MarketRaport(context.Background(), "Golang")
	if err != nil {
		t.Fatal(err)
	}
	if res.ScopeKey != "Golang" {
		t.Errorf("scopeKey = %q, want Golang", res.ScopeKey)
	}
	wantGenerated := time.Date(2026, 8, 7, 7, 46, 10, 502430600, time.UTC)
	if !res.GeneratedAt.Equal(wantGenerated) {
		t.Errorf("generatedAt = %v, want %v", res.GeneratedAt, wantGenerated)
	}
	if len(res.Years) != 2 {
		t.Fatalf("years = %d, want 2", len(res.Years))
	}

	y2023 := res.Years[0]
	if y2023.Year != 2023 || y2023.OfferCount != 20 {
		t.Errorf("unexpected year 0: %+v", y2023)
	}
	// Year-level contractType: all-zero including Total (the fixture's
	// documented quirk — see comment above).
	if y2023.ContractType.Total != 0 || y2023.ContractType.B2BOnly.Count != 0 {
		t.Errorf("unexpected 2023 contractType: %+v", y2023.ContractType)
	}
	// Nested seniority.regular.contractType IS populated despite the node's
	// own count/percentage being zero — pins PermanentOnly/Both, not just B2BOnly.
	rc := y2023.Seniority.Regular.ContractType
	if rc.B2BOnly.Count != 10 || rc.B2BOnly.Percentage != 100 ||
		rc.PermanentOnly.Count != 0 || rc.PermanentOnly.Percentage != 0 ||
		rc.Both.Count != 0 || rc.Both.Percentage != 0 || rc.Total != 10 {
		t.Errorf("unexpected 2023 seniority.regular.contractType: %+v", rc)
	}
	if y2023.SalaryB2B == nil || y2023.SalaryB2B.Regular.MedianLower != 22250 {
		t.Fatalf("unexpected salaryB2B: %+v", y2023.SalaryB2B)
	}
	// Pin the fields TestMarketRaportDecodes previously left unasserted:
	// AverageLower/AverageUpper/SalaryRangeCount, and the Senior band.
	if b := y2023.SalaryB2B.Regular; b.AverageLower != 23290 || b.AverageUpper != 30430 || b.SalaryRangeCount != 10 {
		t.Errorf("unexpected 2023 salaryB2B.regular averages/count: %+v", b)
	}
	if b := y2023.SalaryB2B.Senior; b.MedianLower != 24350 || b.MedianUpper != 30350 ||
		b.AverageLower != 22450 || b.AverageUpper != 29640 || b.SalaryRangeCount != 10 {
		t.Errorf("unexpected 2023 salaryB2B.senior: %+v", b)
	}
	if y2023.SalaryPermanent != nil {
		t.Errorf("salaryUoP should decode to nil SalaryPermanent for 2023, got %+v", y2023.SalaryPermanent)
	}
	if len(y2023.TopSkills) != 2 || y2023.TopSkills[0].Name != "Golang" || y2023.TopSkills[0].Count != 20 {
		t.Errorf("unexpected topSkills: %+v", y2023.TopSkills)
	}

	y2024 := res.Years[1]
	if y2024.ContractType.Total != 43 || y2024.ContractType.B2BOnly.Percentage != 88 ||
		y2024.ContractType.PermanentOnly.Count != 4 || y2024.ContractType.Both.Count != 1 {
		t.Errorf("unexpected contractType: %+v", y2024.ContractType)
	}
	if y2024.Seniority.Regular.Count != 22 || y2024.Seniority.Regular.Percentage != 51 ||
		y2024.Seniority.Regular.ContractType.Total != 22 {
		t.Errorf("unexpected seniority.regular: %+v", y2024.Seniority.Regular)
	}
	// Junior and Senior nodes were never asserted before — pin them too.
	if y2024.Seniority.Junior.Count != 0 || y2024.Seniority.Junior.Percentage != 0 {
		t.Errorf("unexpected seniority.junior: %+v", y2024.Seniority.Junior)
	}
	if y2024.Seniority.Senior.Count != 21 || y2024.Seniority.Senior.Percentage != 49 ||
		y2024.Seniority.Senior.ContractType.Both.Count != 1 {
		t.Errorf("unexpected seniority.senior: %+v", y2024.Seniority.Senior)
	}
	if y2024.SalaryPermanent == nil || y2024.SalaryPermanent.Senior.MedianUpper != 22000 {
		t.Fatalf("unexpected salaryUoP/SalaryPermanent: %+v", y2024.SalaryPermanent)
	}
	if b := y2024.SalaryPermanent.Regular; b.MedianLower != 14500 || b.AverageLower != 14500 ||
		b.AverageUpper != 18500 || b.SalaryRangeCount != 2 {
		t.Errorf("unexpected 2024 salaryUoP.regular: %+v", b)
	}

	if len(y2024.Quarters) != 2 {
		t.Fatalf("quarters = %d, want 2", len(y2024.Quarters))
	}
	q1 := y2024.Quarters[0]
	if q1.Quarter != 1 || q1.OfferCount != 11 || q1.ContractType.Total != 11 {
		t.Errorf("unexpected 2024 Q1: %+v", q1)
	}
	if q1.SalaryB2B == nil || q1.SalaryB2B.Regular.MedianLower != 21000 {
		t.Errorf("unexpected 2024 Q1 salaryB2B: %+v", q1.SalaryB2B)
	}
	// Q2's fixture omits the salaryUoP key entirely — must decode to nil,
	// not zero-value, same as the year-level "salaryUoP":null case above.
	q2 := y2024.Quarters[1]
	if q2.Quarter != 2 || q2.OfferCount != 9 {
		t.Errorf("unexpected 2024 Q2: %+v", q2)
	}
	if q2.SalaryPermanent != nil {
		t.Errorf("Q2 salaryUoP should decode to nil SalaryPermanent, got %+v", q2.SalaryPermanent)
	}
	if len(y2023.Quarters) != 0 {
		t.Errorf("2023 quarters = %d, want 0 (fixture has none)", len(y2023.Quarters))
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
