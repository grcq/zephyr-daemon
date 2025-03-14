package utils

import (
	"os"
	"strings"
)

func Normalize(path string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	newPath := ""
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if p == "." {
			continue
		}

		if p == ".." {
			newPath = strings.Join(strings.Split(newPath, "/")[:len(strings.Split(newPath, "/"))-1], "/")
			continue
		}

		newPath += p + "/"
	}

	newPath = strings.TrimSuffix(newPath, "/")
	path = strings.ReplaceAll(path, "/", "\\")
	path = strings.Replace(path, "~", homeDir, 1)
	return path
}
