package middleware

import (
	"daemon/server"
	"github.com/gin-gonic/gin"
	"net/http"
)

func ServerRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Params.Get("server"); !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Server ID is required"})
			return
		}

		serverId := c.Param("server")
		if serverId == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Server ID is required"})
			return
		}

		if s, err := server.GetServer(serverId); err != nil || s == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		} else {
			c.Set("server", s)
		}

		c.Next()
	}
}
