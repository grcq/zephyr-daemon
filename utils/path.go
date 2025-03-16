package utils

import (
	"os"
	"regexp"
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

	d, _ := os.Getwd()
	if !regexp.MustCompile(`^[A-Z]:`).MatchString(newPath) {
		if !strings.HasPrefix(newPath, "/") {
			d += "/"
		}
		newPath = d + newPath
	}

	newPath = strings.TrimSuffix(newPath, "/")
	newPath = strings.ReplaceAll(newPath, "/", "\\")
	newPath = strings.Replace(newPath, "~", homeDir, 1)
	return newPath
}
