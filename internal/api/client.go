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
	"sort"
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

// MarketStatistics fetches aggregated market statistics for a single scope.
// scopeKind is one of ScopeKinds; scopeKey names a value within it (e.g.
// "subcategory"/"React", "city"/"warszawa"). fields is an optional subset of
// MarketSections — nil/empty returns all sections available for the scope.
func (c *Client) MarketStatistics(ctx context.Context, scopeKind, scopeKey string, fields []string) (*MarketStats, error) {
	if !ValidCampaign(c.Campaign) {
		return nil, ErrInvalidCampaign
	}
	if !ValidScopeKind(scopeKind) {
		return nil, fmt.Errorf("invalid scope kind %q: valid kinds are %v", scopeKind, ScopeKinds)
	}
	if scopeKey == "" {
		return nil, errors.New("scope key must not be empty")
	}

	u, err := c.buildMarketStatsURL(scopeKind, scopeKey, fields)
	if err != nil {
		return nil, err
	}

	body, err := c.doWithRetry(ctx, u)
	if err != nil {
		return nil, err
	}

	var out MarketStats
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// MarketRaport fetches the yearly market report for a single role. Unlike
// MarketStatistics there is no scopeKind and no fields filter — the report
// always comes back whole, up to 3 calendar years, oldest first.
func (c *Client) MarketRaport(ctx context.Context, scopeKey string) (*MarketRaport, error) {
	if !ValidCampaign(c.Campaign) {
		return nil, ErrInvalidCampaign
	}
	if scopeKey == "" {
		return nil, errors.New("scope key must not be empty")
	}

	u, err := c.buildMarketRaportURL(scopeKey)
	if err != nil {
		return nil, err
	}

	body, err := c.doWithRetry(ctx, u)
	if err != nil {
		return nil, err
	}

	var out MarketRaport
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	// The API documents "oldest first" but doesn't guarantee ordering in the
	// wire format; sort defensively so callers can rely on Years[0] being the
	// earliest year regardless.
	sort.Slice(out.Years, func(i, j int) bool { return out.Years[i].Year < out.Years[j].Year })
	return &out, nil
}

// buildMarketRaportURL assembles the market-statistics/raport request URL
// with just the campaign param — no scopeKind, no fields.
func (c *Client) buildMarketRaportURL(scopeKey string) (string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	u, err := url.Parse(base + "/market-statistics/raport/" + url.PathEscape(scopeKey))
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("campaign", c.Campaign)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildMarketStatsURL assembles the market-statistics request URL with the
// campaign and an optional comma-separated fields filter.
func (c *Client) buildMarketStatsURL(scopeKind, scopeKey string, fields []string) (string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	u, err := url.Parse(base + "/market-statistics/" + url.PathEscape(scopeKind) + "/" + url.PathEscape(scopeKey))
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("campaign", c.Campaign)
	if v := strings.Join(fields, ","); v != "" {
		q.Set("fields", v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
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
