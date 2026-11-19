package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/snmp161/melisa/internal/model"
)

// WriteJSON serializes the report as indented JSON.
// If path is "-" or empty, writes to stdout.
func WriteJSON(report *model.Report, path string) (err error) {
	var w io.Writer = os.Stdout
	if path != "" && path != "-" {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("close output file: %w", closeErr)
			}
		}()
		w = f
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}
