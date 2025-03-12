package utils

import (
	"os"
	"strings"
)

func Normalize(path string) string {
	path = strings.ReplaceAll(path, "/", "\\")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	// todo: normalize . and ..

	path = strings.Replace(path, "~", homeDir, 1)
	return path
}
