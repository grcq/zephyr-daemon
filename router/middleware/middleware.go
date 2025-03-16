package middleware

import (
	"daemon/server"
	"github.com/gin-gonic/gin"
	"net/http"
)

func ServerExists() gin.HandlerFunc {
	return func(c *gin.Context) {
		var s *server.Server
		if id, ok := c.Params.Get("server"); ok {
			var err error
			if s, err = server.GetServer(id); err != nil {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Server not found"})
				return
			}
		}

		if s == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Server not found"})
			return
		}

		c.Set("server", s)
		c.Next()
	}
}
