// internal/adapters/calibre/columns.go
//
// Custom column management for #readsync_* columns.
// Uses `calibredb custom_columns` to list and `calibredb add_custom_column` to create.

package calibre

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ColumnDef describes a Calibre custom column.
type ColumnDef struct {
	// Name is the column lookup name (e.g. "#readsync_progress").
	Name string
	// Label is used as the column label in the Calibre UI.
	Label string
	// DataType is the calibredb data type: "int", "text", "datetime", "enumeration".
	DataType string
	// Values is a comma-separated list of allowed values for enumerated columns.
	Values string
	// IsMultiple whether the column allows multiple values (false for all readsync cols).
	IsMultiple bool
}

// requiredColumns is the authoritative list of columns ReadSync requires.
var requiredColumns = []ColumnDef{
	{Name: "#readsync_progress", Label: "ReadSync Progress", DataType: "int"},
	{Name: "#readsync_progress_mode", Label: "ReadSync Progress Mode", DataType: "enumeration", Values: "percent,page,raw"},
	{Name: "#readsync_status", Label: "ReadSync Status", DataType: "enumeration", Values: "not_started,reading,finished,abandoned"},
	{Name: "#readsync_last_position", Label: "ReadSync Last Position", DataType: "text"},
	{Name: "#readsync_last_source", Label: "ReadSync Last Source", DataType: "text"},
	{Name: "#readsync_last_synced", Label: "ReadSync Last Synced", DataType: "datetime"},
	{Name: "#readsync_conflict", Label: "ReadSync Conflict", DataType: "text"},
	{Name: "#readsync_confidence", Label: "ReadSync Confidence", DataType: "int"},
}

// optionalColumns are created only if the user opts in (not gated by health check).
var optionalColumns = []ColumnDef{
	{Name: "#readsync_goodreads_state", Label: "ReadSync Goodreads State", DataType: "text"},
	{Name: "#readsync_koreader_hash", Label: "ReadSync KOReader Hash", DataType: "text"},
	{Name: "#readsync_moon_key", Label: "ReadSync Moon+ Key", DataType: "text"},
	{Name: "#readsync_raw_locator", Label: "ReadSync Raw Locator", DataType: "text"},
}

// listCustomColumns runs `calibredb custom_columns` and returns the result.
func listCustomColumns(calibredbPath, libraryPath string) ([]string, error) {
	cmd := exec.Command(calibredbPath, "custom_columns",
		"--library-path", libraryPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("calibredb custom_columns: %w", err)
	}
	var cols []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// calibredb custom_columns output format:  "#colname (label) - description"
		// The first field is already the full lookup name including "#" prefix.
		parts := strings.Fields(line)
		if len(parts) > 0 {
			name := parts[0]
			// Ensure it has the # prefix (some Calibre versions may omit it).
			if !strings.HasPrefix(name, "#") {
				name = "#" + name
			}
			cols = append(cols, name)
		}
	}
	return cols, nil
}

// missingColumns returns ColumnDef entries for required columns not yet present.
func missingColumns(calibredbPath, libraryPath string) ([]ColumnDef, error) {
	existing, err := listCustomColumns(calibredbPath, libraryPath)
	if err != nil {
		return nil, err
	}
	existSet := make(map[string]bool, len(existing))
	for _, c := range existing {
		existSet[strings.ToLower(c)] = true
	}
	var missing []ColumnDef
	for _, req := range requiredColumns {
		if !existSet[strings.ToLower(req.Name)] {
			missing = append(missing, req)
		}
	}
	return missing, nil
}

// createColumn creates a single custom column via calibredb add_custom_column.
func createColumn(ctx context.Context, calibredbPath, libraryPath string, col ColumnDef) error {
	// calibredb add_custom_column --label LOOKUP_NAME --name LABEL --datatype TYPE
	// Strip leading "#" for the lookup name (--label) argument.
	lookupName := strings.TrimPrefix(col.Name, "#")
	args := []string{
		"add_custom_column",
		"--library-path", libraryPath,
		"--label", lookupName,
		"--name", col.Label,
		"--datatype", col.DataType,
	}
	if col.Values != "" {
		enumJSON := fmt.Sprintf(`{"enum_values": [%s]}`, quoteEnumValues(col.Values))
		args = append(args, "--display", enumJSON)
	}
	cmd := exec.CommandContext(ctx, calibredbPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add_custom_column %s: %w\n%s", col.Name, err, string(out))
	}
	return nil
}

// quoteEnumValues converts "a,b,c" to `"a","b","c"` for the JSON column-details arg.
func quoteEnumValues(vals string) string {
	parts := strings.Split(vals, ",")
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			quoted = append(quoted, `"`+p+`"`)
		}
	}
	return strings.Join(quoted, ",")
}
