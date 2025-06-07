package router

import (
	"daemon/server"
	"github.com/gin-gonic/gin"
	"net/http"
)

func getServers(c *gin.Context) {
	c.JSON(http.StatusOK, server.Servers)
}

func getServerStats(c *gin.Context) {
	s := c.MustGet("server").(*server.Server)

	stats, err := s.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get server stats: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
