package cloudsql

import (
	"encoding/json"
	"io"
)

// FormatJSON writes the check result as JSON
func FormatJSON(w io.Writer, result *CheckResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
