// Package api is a typed client for the SOLID.Jobs public offers API
// (https://solid.jobs/public-api). The API needs no auth token, only a
// mandatory campaign parameter used for traffic analytics.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL  = "https://solid.jobs/public-api"
	defaultCampaign = "solid-jobs-skills"
	apiVersion      = "1.0"
	maxPageSize     = 500
)

// campaignRe enforces the API rule: lowercase letters, digits and hyphens,
// at most 64 characters.
var campaignRe = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// ErrInvalidCampaign is returned when a campaign string violates the API rule.
var ErrInvalidCampaign = errors.New("campaign must be lowercase letters, digits and hyphens, max 64 chars")

// ValidCampaign reports whether s is an acceptable campaign parameter.
func ValidCampaign(s string) bool { return campaignRe.MatchString(s) }

// Client talks to the offers endpoint.
type Client struct {
	BaseURL    string
	Campaign   string
	HTTP       *http.Client
	MaxRetries int // retries on HTTP 429 (default 4)
}

// NewClient returns a Client with sensible defaults. Pass an empty campaign to
// use the built-in default.
func NewClient(campaign string) *Client {
	if campaign == "" {
		campaign = defaultCampaign
	}
	return &Client{
		BaseURL:    defaultBaseURL,
		Campaign:   campaign,
		HTTP:       &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 4,
	}
}

// SearchParams holds the optional filters, paging and sorting for a query.
// Zero-value fields are omitted from the request.
type SearchParams struct {
	PageIndex     int
	PageSize      int
	SortActive    string
	SortDirection string

	Cities        []string
	Categories    []string
	SubCategories []string
	Experiences   []string
	SearchTerms   []string
	MinimumSalary int
}

// APIError represents a non-2xx response from the API.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("solid.jobs API: status %d: %s", e.Status, strings.TrimSpace(e.Body))
}

// Search fetches one page of offers for the given division.
func (c *Client) Search(ctx context.Context, division string, p SearchParams) (*OffersResponse, error) {
	if !ValidCampaign(c.Campaign) {
		return nil, ErrInvalidCampaign
	}
	if !ValidDivision(division) {
		return nil, fmt.Errorf("invalid division %q: valid divisions are %v", division, Divisions)
	}

	u, err := c.buildURL(division, p)
	if err != nil {
		return nil, err
	}

	body, err := c.doWithRetry(ctx, u)
	if err != nil {
		return nil, err
	}

	var out OffersResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// buildURL assembles the request URL with campaign, paging, sorting and the
// search.* filter parameters.
func (c *Client) buildURL(division string, p SearchParams) (string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	u, err := url.Parse(base + "/offers/" + url.PathEscape(division))
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("campaign", c.Campaign)

	if p.PageIndex > 0 {
		q.Set("pageIndex", strconv.Itoa(p.PageIndex))
	}
	if p.PageSize > 0 {
		size := p.PageSize
		if size > maxPageSize {
			size = maxPageSize
		}
		q.Set("pageSize", strconv.Itoa(size))
	}
	if p.SortActive != "" {
		q.Set("sortActive", p.SortActive)
	}
	if p.SortDirection != "" {
		q.Set("sortDirection", p.SortDirection)
	}

	// Comma-separated multi-value filters.
	if v := strings.Join(p.Cities, ","); v != "" {
		q.Set("search.cities", v)
	}
	if v := strings.Join(p.Categories, ","); v != "" {
		q.Set("search.categories", v)
	}
	if v := strings.Join(p.SubCategories, ","); v != "" {
		q.Set("search.subCategories", v)
	}
	if v := strings.Join(p.Experiences, ","); v != "" {
		q.Set("search.experiences", v)
	}
	if v := strings.Join(p.SearchTerms, ","); v != "" {
		q.Set("search.searchTerm", v)
	}
	if p.MinimumSalary > 0 {
		q.Set("search.minimumSalary", strconv.Itoa(p.MinimumSalary))
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// doWithRetry issues the GET and retries on 429 with exponential backoff,
// honouring a Retry-After header when present.
func (c *Client) doWithRetry(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := backoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Api-Version", apiVersion)
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = &APIError{Status: resp.StatusCode, Body: string(body)}
			if d := parseRetryAfter(resp.Header.Get("Retry-After")); d > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(d):
				}
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
		}
		return body, nil
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, fmt.Errorf("after %d retries: %w", c.MaxRetries, lastErr)
}

// backoff returns the delay before retry attempt n (1-based): 0.5s, 1s, 2s...
func backoff(attempt int) time.Duration {
	d := 500 * time.Millisecond * (1 << (attempt - 1))
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// parseRetryAfter understands the delta-seconds form of the Retry-After header.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
