package diff

import (
	"sg-culturepass-bookstores-changelog/internal/models"
)

// Change represents a single field change
type Change struct {
	Field    string `json:"field"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

// ChangedLocation represents a location with detected changes
type ChangedLocation struct {
	Location models.Location `json:"location"`
	Changes  []Change        `json:"changes"`
}

// Result contains the diff results between old and new location data
type Result struct {
	Added   []models.Location `json:"added"`
	Changed []ChangedLocation `json:"changed"`
}

// HasChanges returns true if there are any additions or changes
func (r *Result) HasChanges() bool {
	return len(r.Added) > 0 || len(r.Changed) > 0
}

// Compute compares old and new location lists and returns differences
func Compute(old, new []models.Location) *Result {
	result := &Result{
		Added:   make([]models.Location, 0),
		Changed: make([]ChangedLocation, 0),
	}

	// Build map of old locations by ID
	oldMap := make(map[string]models.Location)
	for _, loc := range old {
		oldMap[loc.ID] = loc
	}

	// Compare each new location against old data
	for _, newLoc := range new {
		oldLoc, exists := oldMap[newLoc.ID]

		if !exists {
			// New location added
			result.Added = append(result.Added, newLoc)
			continue
		}

		// Check for changes in tracked fields
		changes := detectChanges(oldLoc, newLoc)
		if len(changes) > 0 {
			// Preserve coordinates from old location
			if oldLoc.Latitude != nil && oldLoc.Longitude != nil {
				newLoc.Latitude = oldLoc.Latitude
				newLoc.Longitude = oldLoc.Longitude
			}

			result.Changed = append(result.Changed, ChangedLocation{
				Location: newLoc,
				Changes:  changes,
			})
		}
	}

	return result
}

// detectChanges compares two locations and returns detected changes
func detectChanges(old, new models.Location) []Change {
	var changes []Change

	if old.Name != new.Name {
		changes = append(changes, Change{
			Field:    "name",
			OldValue: old.Name,
			NewValue: new.Name,
		})
	}

	if old.Address != new.Address {
		changes = append(changes, Change{
			Field:    "address",
			OldValue: old.Address,
			NewValue: new.Address,
		})
	}

	if old.PostalCode != new.PostalCode {
		changes = append(changes, Change{
			Field:    "postalCode",
			OldValue: old.PostalCode,
			NewValue: new.PostalCode,
		})
	}

	return changes
}

// NeedsRegeocoding checks if a postal code change requires re-geocoding
func NeedsRegeocoding(changes []Change) bool {
	for _, c := range changes {
		if c.Field == "postalCode" {
			return true
		}
	}
	return false
}

// MergeLocations merges new locations with existing data, preserving coordinates
func MergeLocations(old, new []models.Location, diffResult *Result) []models.Location {
	// Build map of old locations for coordinate lookup
	oldMap := make(map[string]models.Location)
	for _, loc := range old {
		oldMap[loc.ID] = loc
	}

	// Build final list with preserved/updated coordinates
	merged := make([]models.Location, 0, len(new))
	for _, newLoc := range new {
		if oldLoc, exists := oldMap[newLoc.ID]; exists {
			// Preserve existing coordinates if available
			if oldLoc.Latitude != nil && oldLoc.Longitude != nil {
				newLoc.Latitude = oldLoc.Latitude
				newLoc.Longitude = oldLoc.Longitude
			}
		}
		merged = append(merged, newLoc)
	}

	return merged
}
