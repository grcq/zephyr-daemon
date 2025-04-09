package router

import (
	"daemon/server"
	"github.com/gin-gonic/gin"
	"net/http"
)

func getServers(c *gin.Context) {
	c.JSON(http.StatusOK, server.Servers)
}
