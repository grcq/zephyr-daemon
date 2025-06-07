package router

import (
	"daemon/server"
	"github.com/gin-gonic/gin"
)

func getFiles(c *gin.Context) {
	s := c.MustGet("server").(*server.Server)
	path := c.Query("path")

	entries, err := s.ListDirectory(path)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get files: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"entries": entries,
	})
}

func getFileContent(c *gin.Context) {
	s := c.MustGet("server").(*server.Server)
	path := c.Query("path")

	fileName, content, err := s.ReadFileContent(path)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to read file content: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"name":    fileName,
		"content": content,
	})
}

func saveFileContent(c *gin.Context) {
	s := c.MustGet("server").(*server.Server)
	path := c.Query("path")

	var request struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if err := s.WriteFileContent(path, request.Content); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save file content: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "File content saved successfully"})
}
