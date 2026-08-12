package api

import (
	"strings"
	"time"
)

// Division is one of the solid.jobs offer divisions.
type Division string

const (
	DivisionIT          Division = "IT"
	DivisionEngineering Division = "Engineering"
	DivisionMarketing   Division = "Marketing"
	DivisionSales       Division = "Sales"
	DivisionHR          Division = "HR"
	DivisionLogistics   Division = "Logistics"
	DivisionFinances    Division = "Finances"
	DivisionOther       Division = "Other"
)

// Divisions lists every valid division accepted by the API.
var Divisions = []Division{
	DivisionIT, DivisionEngineering, DivisionMarketing, DivisionSales,
	DivisionHR, DivisionLogistics, DivisionFinances, DivisionOther,
}

// ValidDivision reports whether s names a known division (case-sensitive,
// matching the API's expectations).
func ValidDivision(s string) bool {
	for _, d := range Divisions {
		if string(d) == s {
			return true
		}
	}
	return false
}

// Salary describes a compensation band attached to an offer. From/To are
// nullable in the API response.
type Salary struct {
	From           *float64 `json:"from"`
	To             *float64 `json:"to"`
	Currency       string   `json:"currency"`
	Period         string   `json:"period"`
	EmploymentType string   `json:"employmentType"`
}

// NamedLevel is a skill or language with an associated proficiency level.
type NamedLevel struct {
	Name  string `json:"name"`
	Level string `json:"level"`
}

// Offer mirrors a single job object in the API response.
type Offer struct {
	JobOfferKey     string       `json:"jobOfferKey"`
	Title           string       `json:"title"`
	Division        string       `json:"division"`
	Category        string       `json:"category"`
	SubCategory     string       `json:"subCategory"`
	Company         string       `json:"company"`
	CompanyLogoURL  *string      `json:"companyLogoUrl"`
	Salary          *Salary      `json:"salary"`
	SecondarySalary *Salary      `json:"secondarySalary"`
	ContractTime    string       `json:"contractTime"`
	Locations       []string     `json:"locations"`
	Benefits        []string     `json:"benefits"`
	IsRemote        bool         `json:"isRemote"`
	IsHybrid        bool         `json:"isHybrid"`
	URL             string       `json:"url"`
	ExperienceLevel string       `json:"experienceLevel"`
	Skills          []NamedLevel `json:"skills"`
	Languages       []NamedLevel `json:"languages"`
	Description     string       `json:"description"`
	ValidFrom       time.Time    `json:"validFrom"`
	ValidTo         time.Time    `json:"validTo"`
	UpdatedAt       *time.Time   `json:"updatedAt"`
}

// OffersResponse is the paginated envelope returned by the offers endpoint.
type OffersResponse struct {
	PageIndex  int     `json:"pageIndex"`
	PageSize   int     `json:"pageSize"`
	TotalCount int     `json:"totalCount"`
	TotalPages int     `json:"totalPages"`
	Jobs       []Offer `json:"jobs"`
}

// ScopeKinds lists the scope kinds accepted by the market-statistics endpoint.
// scopeKind is case-insensitive server-side; these are the canonical tokens.
var ScopeKinds = []string{"division", "mainCategory", "subcategory", "subcategoryGroup", "city"}

// ValidScopeKind reports whether s names a known market-statistics scope kind
// (case-insensitive, matching the API).
func ValidScopeKind(s string) bool {
	for _, k := range ScopeKinds {
		if strings.EqualFold(k, s) {
			return true
		}
	}
	return false
}

// ScopeKindRaport is the special scopeKind value that routes to the
// market-statistics raport endpoint instead of the regular scope endpoint.
// It is deliberately excluded from ScopeKinds: raport takes a role name (not
// a division/category/city value) as scopeKey, and never accepts fields.
const ScopeKindRaport = "raport"

// IsRaportScopeKind reports whether s names the raport endpoint
// (case-insensitive, matching the API's path segment).
func IsRaportScopeKind(s string) bool { return strings.EqualFold(s, ScopeKindRaport) }

// MarketSections lists the section tokens accepted in the `fields` parameter of
// the market-statistics endpoint.
var MarketSections = []string{"demand", "salary", "experience", "topLocations", "topSkills"}

// ValidMarketSection reports whether s names a known market-statistics section
// token (case-insensitive, matching the API's `fields` filter).
func ValidMarketSection(s string) bool {
	for _, sec := range MarketSections {
		if strings.EqualFold(sec, s) {
			return true
		}
	}
	return false
}

// MarketStats is the flat, stable statistics contract returned by
// GET /public-api/market-statistics/{scopeKind}/{scopeKey}. Optional sections
// are nil when not requested (or unavailable for the scope).
type MarketStats struct {
	ScopeKind        string       `json:"scopeKind"`
	ScopeKey         string       `json:"scopeKey"`
	GeneratedAt      time.Time    `json:"generatedAt"`
	IncludedSections []string     `json:"includedSections"`
	Demand           *DemandStats `json:"demand"`
	Salary           *SalaryStats `json:"salary"`
	Experience       []Bucket     `json:"experience"`
	TopLocations     []Bucket     `json:"topLocations"`
	TopSkills        []Bucket     `json:"topSkills"`
}

// DemandStats holds demand and hiring metrics for a scope.
type DemandStats struct {
	ActiveOffers      int          `json:"activeOffers"`
	DistinctEmployers int          `json:"distinctEmployers"`
	RemoteOffers      int          `json:"remoteOffers"`
	RemotePercentage  int          `json:"remotePercentage"`
	OfferTrend        []TrendPoint `json:"offerTrend"`
}

// TrendPoint is one quarterly point of the offer-count trend.
type TrendPoint struct {
	Period     string `json:"period"`
	OfferCount int    `json:"offerCount"`
}

// SalaryStats holds salary metrics for a scope, in the currency below (PLN
// today). Amounts are assumed monthly: the endpoint carries no interval field,
// so callers comparing an offer's band against these should account for
// offer-level Salary.Period (e.g. hourly/daily B2B rates). Overall/B2B/Permanent
// are nil when absent.
type SalaryStats struct {
	Currency  string      `json:"currency"`
	Overall   *SalaryBand `json:"overall"`
	B2B       *SalaryStat `json:"b2b"`
	Permanent *SalaryStat `json:"permanent"`
}

// SalaryBand is a percentile band computed live from active offers.
type SalaryBand struct {
	Min    float64 `json:"min"`
	P25    float64 `json:"p25"`
	Median float64 `json:"median"`
	P75    float64 `json:"p75"`
	Max    float64 `json:"max"`
}

// SalaryStat is a precomputed salary stat for one contract type.
type SalaryStat struct {
	Median     float64 `json:"median"`
	Average    float64 `json:"average"`
	OfferCount int     `json:"offerCount"`
}

// Bucket is one entry of a distribution (experience / topLocations / topSkills):
// a label, its offer count and its share (0–100) of the scope's active offers.
type Bucket struct {
	Label      string `json:"label"`
	OfferCount int    `json:"offerCount"`
	Percentage int    `json:"percentage"`
}

// MarketRaport is the yearly (with a quarterly breakdown per year) market
// report for a single role, returned by GET
// /public-api/market-statistics/raport/{scopeKey}. Unlike MarketStats there
// is no scopeKind and no fields filter — the report always comes back whole,
// up to 3 calendar years, oldest first.
type MarketRaport struct {
	ScopeKey    string       `json:"scopeKey"`
	GeneratedAt time.Time    `json:"generatedAt"`
	Years       []RaportYear `json:"years"`
}

// RaportYear is one calendar year's slice of a MarketRaport.
type RaportYear struct {
	Year         int                   `json:"year"`
	OfferCount   int                   `json:"offerCount"`
	ContractType ContractTypeBreakdown `json:"contractType"`
	Seniority    SeniorityBreakdown    `json:"seniority"`
	// Nil when the year has no data at all for that contract type.
	SalaryB2B       *RaportSalaryStat `json:"salaryB2B"`
	SalaryPermanent *RaportSalaryStat `json:"salaryUoP"`
	// Most required skills that year, descending by Count. Length (and any cap
	// on it) is determined server-side; this client applies no normalization,
	// so a year with no skill data may decode this as nil rather than empty.
	TopSkills []RaportSkill `json:"topSkills"`
	// Quarterly breakdown of this year, Q1-Q4, oldest first. A partial year
	// (the current, still-in-progress one) may carry fewer than 4 entries.
	Quarters []RaportQuarter `json:"quarters"`
}

// RaportQuarter is one calendar quarter's slice of a RaportYear — same shape
// as the year-level breakdown minus TopSkills, which the API only reports at
// year granularity.
type RaportQuarter struct {
	Quarter      int                   `json:"quarter"`
	OfferCount   int                   `json:"offerCount"`
	ContractType ContractTypeBreakdown `json:"contractType"`
	Seniority    SeniorityBreakdown    `json:"seniority"`
	// Nil when the quarter has no data at all for that contract type — seen
	// in practice for salaryUoP, whose key is sometimes omitted entirely.
	SalaryB2B       *RaportSalaryStat `json:"salaryB2B"`
	SalaryPermanent *RaportSalaryStat `json:"salaryUoP"`
}

// ContractTypeBreakdown's Total is the denominator of every Percentage in the
// breakdown — it is not guaranteed to equal OfferCount (it can coincide, as
// seen in some fixture years, but offers proposing neither B2B nor a
// permanent contract fall outside all three buckets, so don't assume
// equality). Used both at year level and nested inside each SeniorityNode.
type ContractTypeBreakdown struct {
	B2BOnly       CountWithPercentage `json:"b2bOnly"`
	PermanentOnly CountWithPercentage `json:"permanentOnly"`
	Both          CountWithPercentage `json:"both"`
	Total         int                 `json:"total"`
}

// SeniorityBreakdown's Total is the sum of the three levels' Count — it is
// not guaranteed to equal OfferCount. Offers with no declared experience
// level fall outside all three levels.
type SeniorityBreakdown struct {
	Junior  SeniorityNode `json:"junior"`
	Regular SeniorityNode `json:"regular"`
	Senior  SeniorityNode `json:"senior"`
	Total   int           `json:"total"`
}

// SeniorityNode is one experience level's count, percentage, and its own
// independent contract-type split.
type SeniorityNode struct {
	Count        int                   `json:"count"`
	Percentage   int                   `json:"percentage"`
	ContractType ContractTypeBreakdown `json:"contractType"`
}

// CountWithPercentage is a raw count plus its share (0-100) of some
// context-dependent total.
type CountWithPercentage struct {
	Count      int `json:"count"`
	Percentage int `json:"percentage"`
}

// RaportSalaryStat holds salary levels for one contract type in one year,
// keyed by seniority. When present, all three seniority keys are always
// populated — a seniority with no matching offers still appears, with every
// field at zero.
type RaportSalaryStat struct {
	Junior  RaportSalaryBand `json:"junior"`
	Regular RaportSalaryBand `json:"regular"`
	Senior  RaportSalaryBand `json:"senior"`
}

// RaportSalaryBand is a salary band for one seniority level in one year.
// Assumed monthly PLN, same as MarketStats.SalaryStats — the raport response
// carries no currency or interval field of its own, so this can't be
// confirmed from the payload. Figures are a range, not a single point
// estimate — pooled from both ends of every matching salary range. A zero
// band (all fields zero, SalaryRangeCount 0) means no data, not PLN 0 — check
// SalaryRangeCount before treating the figures as real.
type RaportSalaryBand struct {
	MedianLower  float64 `json:"medianLower"`
	MedianUpper  float64 `json:"medianUpper"`
	AverageLower float64 `json:"averageLower"`
	AverageUpper float64 `json:"averageUpper"`
	// Number of salary ranges behind the figures for this seniority — NOT a
	// distinct offer count. All fields are zero when this seniority has no
	// salary data that year.
	SalaryRangeCount int `json:"salaryRangeCount"`
}

// RaportSkill is one skill and how many offers required it in a given year.
type RaportSkill struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
