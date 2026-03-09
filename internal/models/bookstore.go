package models

// APIResponse represents the raw response from CulturePass API
type APIResponse struct {
	Data struct {
		Bookstores []Bookstore         `json:"bookstores"`
		Outlets    map[string][]Outlet `json:"outlets"`
	} `json:"data"`
}

// Bookstore represents a bookstore entry from the API
type Bookstore struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ChineseName    string `json:"chinese-name"`
	MalayName      string `json:"malay-name"`
	TamilName      string `json:"tamil-name"`
	CoverImageURL  string `json:"coverImageUrl"`
	Address        string `json:"address,omitempty"`
	PostalCode     string `json:"postalCode,omitempty"`
	NumOfLocations int    `json:"numOfLocations,omitempty"`
}

// Outlet represents a specific outlet/branch of a bookstore
type Outlet struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ChineseName       string `json:"chinese-name"`
	MalayName         string `json:"malay-name"`
	TamilName         string `json:"tamil-name"`
	OutletName        string `json:"outletName"`
	ChineseOutletName string `json:"chinese-outletName"`
	MalayOutletName   string `json:"malay-outletName"`
	TamilOutletName   string `json:"tamil-outletName"`
	Address           string `json:"address"`
	PostalCode        string `json:"postalCode"`
	CoverImageURL     string `json:"coverImageUrl"`
}

// Location represents a flattened bookstore location for tracking and map display
type Location struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Address    string   `json:"address"`
	PostalCode string   `json:"postalCode"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
}
