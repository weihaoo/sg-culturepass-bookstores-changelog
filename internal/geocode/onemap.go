package geocode

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	OneMapSearchURL = "https://www.onemap.gov.sg/api/common/elastic/search"
	// RequestTimeout for OneMap API calls
	RequestTimeout = 10 * time.Second
	// RateLimitDelay between requests to respect rate limits
	RateLimitDelay = 100 * time.Millisecond
)

// Coordinates represents latitude and longitude
type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// OneMapResponse represents the response from OneMap search API
type OneMapResponse struct {
	Found      int             `json:"found"`
	TotalPages int             `json:"totalNumPages"`
	PageNum    int             `json:"pageNum"`
	Results    []OneMapResult  `json:"results"`
}

// OneMapResult represents a single result from OneMap search
type OneMapResult struct {
	SearchVal  string `json:"SEARCHVAL"`
	Postal     string `json:"POSTAL"`
	Latitude   string `json:"LATITUDE"`
	Longitude  string `json:"LONGITUDE"`
	Address    string `json:"ADDRESS"`
	BuildingName string `json:"BUILDING"`
}

// Client handles geocoding requests to OneMap API
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new OneMap geocoding client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: RequestTimeout,
		},
	}
}

// GeocodePostalCode looks up coordinates for a Singapore postal code
func (c *Client) GeocodePostalCode(postalCode string) (*Coordinates, error) {
	// Build request URL
	params := url.Values{}
	params.Set("searchVal", postalCode)
	params.Set("returnGeom", "Y")
	params.Set("getAddrDetails", "Y")
	params.Set("pageNum", "1")

	reqURL := fmt.Sprintf("%s?%s", OneMapSearchURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "SG-CulturePass-Bookstores-Tracker/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OneMap API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var oneMapResp OneMapResponse
	if err := json.Unmarshal(body, &oneMapResp); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}

	if oneMapResp.Found == 0 || len(oneMapResp.Results) == 0 {
		return nil, fmt.Errorf("no results found for postal code: %s", postalCode)
	}

	// Use the first result
	result := oneMapResp.Results[0]

	lat, err := strconv.ParseFloat(result.Latitude, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing latitude: %w", err)
	}

	lng, err := strconv.ParseFloat(result.Longitude, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing longitude: %w", err)
	}

	return &Coordinates{
		Latitude:  lat,
		Longitude: lng,
	}, nil
}

// RateLimit adds a delay to respect API rate limits
func RateLimit() {
	time.Sleep(RateLimitDelay)
}
