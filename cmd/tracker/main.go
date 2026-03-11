package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"sg-culturepass-bookstores-changelog/internal/api"
	"sg-culturepass-bookstores-changelog/internal/changelog"
	"sg-culturepass-bookstores-changelog/internal/diff"
	"sg-culturepass-bookstores-changelog/internal/geocode"
	"sg-culturepass-bookstores-changelog/internal/models"
)

const (
	dataFilePath   = "data/bookstores.json"
	readmeFilePath = "README.md"
)

func main() {
	log.Println("Starting SG CulturePass Bookstores Tracker...")

	// Step 1: Fetch bookstores from API
	log.Println("Fetching bookstores from CulturePass API...")
	apiClient := api.NewClient()
	apiResp, err := apiClient.FetchBookstores()
	if err != nil {
		log.Fatalf("Failed to fetch bookstores: %v", err)
	}

	// Step 2: Flatten locations
	log.Println("Processing location data...")
	newLocations := api.FlattenLocations(apiResp)
	log.Printf("Found %d locations in API response", len(newLocations))

	// Step 3: Load previous state
	log.Println("Loading previous state...")
	oldLocations, err := loadLocations(dataFilePath)
	if err != nil {
		log.Printf("No previous state found (first run): %v", err)
		oldLocations = []models.Location{}
	} else {
		log.Printf("Loaded %d locations from previous state", len(oldLocations))
	}

	// Step 4: Compute diff
	log.Println("Computing differences...")
	diffResult := diff.Compute(oldLocations, newLocations)

	if !diffResult.HasChanges() {
		log.Println("No changes detected. Exiting.")
		return
	}

	log.Printf("Detected %d new locations and %d changed locations",
		len(diffResult.Added), len(diffResult.Changed))

	// Step 5: Geocode new locations
	if len(diffResult.Added) > 0 {
		log.Println("Geocoding new locations...")
		geocodeClient := geocode.NewClient()
		geocodeNewLocations(geocodeClient, diffResult.Added)
	}

	// Step 6: Handle changed locations (re-geocode if postal code changed)
	if len(diffResult.Changed) > 0 {
		log.Println("Processing changed locations...")
		geocodeClient := geocode.NewClient()
		geocodeChangedLocations(geocodeClient, diffResult.Changed, oldLocations)
	}

	// Step 7: Merge locations with coordinates
	mergedLocations := mergeWithCoordinates(oldLocations, newLocations, diffResult)

	// Step 8: Update README changelog
	log.Println("Updating changelog in README.md...")
	entry := changelog.FormatEntry(time.Now(), diffResult, len(oldLocations), len(newLocations))
	if err := changelog.UpdateReadme(readmeFilePath, entry); err != nil {
		log.Fatalf("Failed to update README: %v", err)
	}

	// Step 9: Save new state
	log.Println("Saving updated location data...")
	if err := saveLocations(dataFilePath, mergedLocations); err != nil {
		log.Fatalf("Failed to save locations: %v", err)
	}

	log.Println("Tracker completed successfully!")
	log.Printf("Summary: +%d new, ~%d changed", len(diffResult.Added), len(diffResult.Changed))
}

// loadLocations reads the locations JSON file
func loadLocations(path string) ([]models.Location, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var locations []models.Location
	if err := json.Unmarshal(data, &locations); err != nil {
		return nil, fmt.Errorf("parsing locations JSON: %w", err)
	}

	return locations, nil
}

// saveLocations writes the locations to a JSON file
func saveLocations(path string, locations []models.Location) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(locations, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling locations: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// geocodeNewLocations fetches coordinates for new locations
func geocodeNewLocations(client *geocode.Client, locations []models.Location) {
	for i := range locations {
		loc := &locations[i]
		if loc.PostalCode == "" {
			log.Printf("  Skipping %s (no postal code)", loc.Name)
			continue
		}

		coords, err := client.GeocodePostalCode(loc.PostalCode)
		if err != nil {
			log.Printf("  Failed to geocode %s (%s): %v", loc.Name, loc.PostalCode, err)
			continue
		}

		loc.Latitude = &coords.Latitude
		loc.Longitude = &coords.Longitude
		log.Printf("  Geocoded %s: %.6f, %.6f", loc.Name, coords.Latitude, coords.Longitude)

		// Rate limiting
		geocode.RateLimit()
	}
}

// geocodeChangedLocations re-geocodes locations where postal code changed
func geocodeChangedLocations(client *geocode.Client, changed []diff.ChangedLocation, oldLocations []models.Location) {
	// Build old locations map for coordinate lookup
	oldMap := make(map[string]models.Location)
	for _, loc := range oldLocations {
		oldMap[loc.ID] = loc
	}

	for i := range changed {
		c := &changed[i]
		
		// Check if postal code changed
		if diff.NeedsRegeocoding(c.Changes) {
			coords, err := client.GeocodePostalCode(c.Location.PostalCode)
			if err != nil {
				log.Printf("  Failed to re-geocode %s: %v", c.Location.Name, err)
				continue
			}

			c.Location.Latitude = &coords.Latitude
			c.Location.Longitude = &coords.Longitude
			log.Printf("  Re-geocoded %s: %.6f, %.6f", c.Location.Name, coords.Latitude, coords.Longitude)

			geocode.RateLimit()
		} else {
			// Preserve existing coordinates
			if old, exists := oldMap[c.Location.ID]; exists {
				c.Location.Latitude = old.Latitude
				c.Location.Longitude = old.Longitude
			}
		}
	}
}

// mergeWithCoordinates creates final location list with all coordinates
func mergeWithCoordinates(old, new []models.Location, diffResult *diff.Result) []models.Location {
	// Build maps for quick lookup
	oldMap := make(map[string]models.Location)
	for _, loc := range old {
		oldMap[loc.ID] = loc
	}

	addedMap := make(map[string]models.Location)
	for _, loc := range diffResult.Added {
		addedMap[loc.ID] = loc
	}

	changedMap := make(map[string]models.Location)
	for _, c := range diffResult.Changed {
		changedMap[c.Location.ID] = c.Location
	}

	// Build final list
	result := make([]models.Location, 0, len(new))
	for _, loc := range new {
		// Check if it's a newly added location with coordinates
		if added, exists := addedMap[loc.ID]; exists {
			result = append(result, added)
			continue
		}

		// Check if it's a changed location
		if changed, exists := changedMap[loc.ID]; exists {
			result = append(result, changed)
			continue
		}

		// Unchanged location - preserve coordinates from old data
		if oldLoc, exists := oldMap[loc.ID]; exists {
			loc.Latitude = oldLoc.Latitude
			loc.Longitude = oldLoc.Longitude
		}
		result = append(result, loc)
	}

	return result
}