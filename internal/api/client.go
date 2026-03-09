package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sg-culturepass-bookstores-changelog/internal/models"
)

const (
	// DefaultAPIURL is the CulturePass bookstores API endpoint
	DefaultAPIURL = "https://api.sgculturepass.gov.sg/v1/bookstores?sortBy=AlphabeticallyAToZ"
	// RequestTimeout is the HTTP request timeout
	RequestTimeout = 30 * time.Second
)

// Client handles API requests to the CulturePass API
type Client struct {
	httpClient *http.Client
	apiURL     string
}

// NewClient creates a new API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: RequestTimeout,
		},
		apiURL: DefaultAPIURL,
	}
}

// FetchBookstores fetches bookstore data from the CulturePass API
func (c *Client) FetchBookstores() (*models.APIResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers 
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://www.sgculturepass.gov.sg")
	req.Header.Set("Referer", "https://www.sgculturepass.gov.sg/")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var apiResp models.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}

	return &apiResp, nil
}

// FlattenLocations converts API response into a flat list of locations
func FlattenLocations(resp *models.APIResponse) []models.Location {
	locations := make([]models.Location, 0)
	seenIDs := make(map[string]bool)

	// Process single-location bookstores (those with address directly in bookstores array)
	for _, bs := range resp.Data.Bookstores {
		if bs.Address != "" && bs.PostalCode != "" {
			if seenIDs[bs.ID] {
				continue
			}
			seenIDs[bs.ID] = true

			locations = append(locations, models.Location{
				ID:         bs.ID,
				Name:       bs.Name,
				Address:    cleanAddress(bs.Address),
				PostalCode: bs.PostalCode,
			})
		}
	}

	// Process outlets from multi-location bookstores
	for brandName, outlets := range resp.Data.Outlets {
		for _, outlet := range outlets {
			if seenIDs[outlet.ID] {
				continue
			}
			seenIDs[outlet.ID] = true

			// Format name as "Brand - Outlet Name"
			displayName := formatOutletName(brandName, outlet.OutletName)

			locations = append(locations, models.Location{
				ID:         outlet.ID,
				Name:       displayName,
				Address:    cleanAddress(outlet.Address),
				PostalCode: outlet.PostalCode,
			})
		}
	}

	return locations
}

// formatOutletName creates a display name combining brand and outlet names
func formatOutletName(brandName, outletName string) string {
	// Clean up outlet name (remove newlines and extra whitespace)
	cleanOutlet := strings.TrimSpace(outletName)
	cleanOutlet = strings.ReplaceAll(cleanOutlet, "\n", " ")
	cleanOutlet = strings.ReplaceAll(cleanOutlet, "\r", "")
	cleanOutlet = strings.Join(strings.Fields(cleanOutlet), " ")

	// If outlet name already contains brand name, just use outlet name
	if strings.Contains(strings.ToLower(cleanOutlet), strings.ToLower(brandName)) {
		return cleanOutlet
	}

	return fmt.Sprintf("%s - %s", brandName, cleanOutlet)
}

// cleanAddress removes extra whitespace and newlines from address
func cleanAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.ReplaceAll(addr, "\n", " ")
	addr = strings.ReplaceAll(addr, "\r", "")
	addr = strings.Join(strings.Fields(addr), " ")
	return addr
}
