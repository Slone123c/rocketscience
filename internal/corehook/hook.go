package corehook

import (
	"path/filepath"
	"strings"
)

// Command is shared by the launcher and exact process-ownership checks.
func Command(dir string) string {
	path := filepath.Join(dir, "core.ready")
	return "printf ready > '" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
