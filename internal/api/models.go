package api

import "time"

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
