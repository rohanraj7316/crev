// Contains code to write content to files.
package files

import (
	"fmt"
	"os"
)

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Saves a string to a file.
func SaveStringToFile(content string, path string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	// https://trstringer.com/golang-deferred-function-error-handling/
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("failed to close file: %w", closeErr)
			}
		}
	}()
	_, err = f.WriteString(content)
	if err != nil {
		return err
	}
	return nil
}
