package server

import (
	"daemon/config"
	"daemon/utils"
	"errors"
	"github.com/apex/log"
	"os"
	"strings"
)

func (s *Server) ReadFileContent(path string) (string, string, error) {
	c := config.Get()
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	absPath := utils.Normalize(c.VolumesPath + "/" + s.Uuid + path)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", "", errors.New("file does not exist: " + absPath)
	}

	split := strings.Split(absPath, "/")
	fileName := split[len(split)-1]

	content, err := os.ReadFile(absPath)
	if err != nil {
		log.WithError(err).Errorf("failed to read file %s", absPath)
		return fileName, "", err
	}

	return fileName, string(content), nil
}

func (s *Server) WriteFileContent(path string, content string) error {
	c := config.Get()
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	absPath := utils.Normalize(c.VolumesPath + "/" + s.Uuid + path)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return err
		}
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return err
	}

	return nil
}

type FileEntry struct {
	Name         string `json:"name"`
	LastModified string `json:"last_modified"`
	Size         int64  `json:"size"`
	IsDir        bool   `json:"is_dir"`
}

func (s *Server) ListDirectory(path string) ([]FileEntry, error) {
	c := *config.Get()
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	absPath := utils.Normalize(c.VolumesPath + "/" + s.Uuid + path)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, errors.New("directory does not exist: " + absPath)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, FileEntry{
			Name:         entry.Name(),
			LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
			Size:         info.Size(),
			IsDir:        entry.IsDir(),
		})
	}

	// sort the files by name and if directory, put directories first

	return files, nil
}
