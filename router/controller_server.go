package router

import (
	"daemon/server"
	"github.com/apex/log"
	"github.com/gin-gonic/gin"
	"net/http"
)

func getServers(c *gin.Context) {
	s, err := server.GetServers()
	if err != nil {
		log.WithError(err).Error("Failed to get templates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get templates"})
		return
	}

	c.JSON(http.StatusOK, s)
}
