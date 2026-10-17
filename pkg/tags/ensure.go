package tags

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultDeprecationsBody = `# Tag deprecations. Each [[deprecated]] maps legacy "from" to preferred "to".
# Example:
# [[deprecated]]
# from = "old-id"
# to = "new-id"
#
# [[dropped]]
# id = "removed-id"
`

// EnsureFiles creates deprecations when missing. Does not overwrite an existing register.
func EnsureFiles(registerPath, deprecationsPath, seedPath string) error {
	const op = "tags.EnsureFiles"
	seedPath = strings.TrimSpace(seedPath)
	if seedPath == "" {
		return newErr(op, CodeInvalid, "empty seed path", nil)
	}
	seed, err := os.ReadFile(seedPath)
	if err != nil {
		return newErr(op, CodeFailed, "read seed", err)
	}
	if strings.TrimSpace(string(seed)) == "" {
		return newErr(op, CodeFailed, "seed file empty", nil)
	}
	if err := copyIfMissing(deprecationsPath, []byte(defaultDeprecationsBody)); err != nil {
		return newErr(op, CodeFailed, "ensure deprecations", err)
	}
	registerPath = filepath.Clean(strings.TrimSpace(registerPath))
	if registerPath != "" {
		if _, statErr := os.Stat(registerPath); statErr != nil && !os.IsNotExist(statErr) {
			return newErr(op, CodeFailed, "stat register", statErr)
		}
	}
	return nil
}

func copyIfMissing(dest string, body []byte) error {
	const op = "tags.copyIfMissing"
	dest = filepath.Clean(strings.TrimSpace(dest))
	if dest == "" {
		return nil
	}
	_, statErr := os.Stat(dest)
	if statErr == nil {
		return nil
	}
	if !os.IsNotExist(statErr) {
		return newErr(op, CodeFailed, "stat dest", statErr)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return newErr(op, CodeFailed, "mkdir dest dir", err)
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return newErr(op, CodeFailed, "write tmp", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return newErr(op, CodeFailed, "rename", err)
	}
	return nil
}
