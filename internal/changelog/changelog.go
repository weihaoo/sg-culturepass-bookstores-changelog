package changelog

import (
	"fmt"
	"os"
	"strings"
	"time"

	"sg-culturepass-bookstores-changelog/internal/diff"
	"sg-culturepass-bookstores-changelog/internal/models"
)

const (
	// ChangelogStartMarker marks where changelog entries begin
	ChangelogStartMarker = "<!-- CHANGELOG_START -->"
	// ChangelogEndMarker marks where changelog entries end
	ChangelogEndMarker = "<!-- CHANGELOG_END -->"
)

// FormatEntry generates a markdown changelog entry with collapsible sections and tables
func FormatEntry(date time.Time, result *diff.Result, oldCount, newCount int) string {
	var sb strings.Builder

	dateStr := date.Format("2006-01-02")
	addedCount := len(result.Added)
	changedCount := len(result.Changed)

	// Outer collapsible section for the date
	sb.WriteString(fmt.Sprintf("<details>\n<summary><strong>%s</strong></summary>\n\n", dateStr))

	// Total locations line
	sb.WriteString(fmt.Sprintf("Total locations: %d → %d\n\n", oldCount, newCount))

	// Added section (collapsible)
	if addedCount > 0 {
		sb.WriteString(fmt.Sprintf("<details>\n<summary>Added (%d)</summary>\n\n", addedCount))
		sb.WriteString(formatAddedTable(result.Added))
		sb.WriteString("\n</details>\n\n")
	}

	// Changed section (collapsible)
	if changedCount > 0 {
		sb.WriteString(fmt.Sprintf("<details>\n<summary>Changed (%d)</summary>\n\n", changedCount))
		sb.WriteString(formatChangedTable(result.Changed))
		sb.WriteString("\n</details>\n\n")
	}

	// Close outer details
	sb.WriteString("</details>\n\n")

	return sb.String()
}

// formatAddedTable creates a markdown table for added locations
func formatAddedTable(locations []models.Location) string {
	var sb strings.Builder

	// Table header
	sb.WriteString("| Name | Address | Postal Code |\n")
	sb.WriteString("|------|---------|-------------|\n")

	// Table rows
	for _, loc := range locations {
		name := escapeTableCell(loc.Name)
		address := escapeTableCell(loc.Address)
		postalCode := loc.PostalCode

		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, address, postalCode))
	}

	return sb.String()
}

// formatChangedTable creates a markdown table for changed locations
func formatChangedTable(changed []diff.ChangedLocation) string {
	var sb strings.Builder

	// Table header
	sb.WriteString("| Name | Address | Postal Code |\n")
	sb.WriteString("|------|---------|-------------|\n")

	// Table rows
	for _, c := range changed {
		nameCell := formatChangedCell("name", c.Location.Name, c.Changes)
		addressCell := formatChangedCell("address", c.Location.Address, c.Changes)
		postalCodeCell := formatChangedCell("postalCode", c.Location.PostalCode, c.Changes)

		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", nameCell, addressCell, postalCodeCell))
	}

	return sb.String()
}

// formatChangedCell formats a table cell, showing old/new if the field changed
func formatChangedCell(fieldName, newValue string, changes []diff.Change) string {
	for _, c := range changes {
		if c.Field == fieldName {
			oldVal := escapeTableCell(c.OldValue)
			newVal := escapeTableCell(c.NewValue)
			// Show strikethrough old value, line break, then new value
			return fmt.Sprintf("~~%s~~ <br> %s", oldVal, newVal)
		}
	}
	// Field didn't change, just show current value
	return escapeTableCell(newValue)
}

// escapeTableCell escapes special characters for markdown table cells
func escapeTableCell(s string) string {
	// Replace pipe characters which break table formatting
	s = strings.ReplaceAll(s, "|", "\\|")
	// Replace newlines with space
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	// Trim extra whitespace
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// UpdateReadme reads the README file, inserts the changelog entry, and writes it back
func UpdateReadme(readmePath string, entry string) error {
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("reading README: %w", err)
	}

	contentStr := string(content)

	// Find the changelog start marker
	startIdx := strings.Index(contentStr, ChangelogStartMarker)
	if startIdx == -1 {
		return fmt.Errorf("changelog start marker not found in README")
	}

	// Insert entry after the start marker
	insertPos := startIdx + len(ChangelogStartMarker) + 1 // +1 for newline

	newContent := contentStr[:insertPos] + entry + contentStr[insertPos:]

	if err := os.WriteFile(readmePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("writing README: %w", err)
	}

	return nil
}
