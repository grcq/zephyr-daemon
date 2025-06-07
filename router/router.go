package router

import (
	"daemon/router/middleware"
	"github.com/apex/log"
	"github.com/gin-gonic/gin"
	"net/http"
)

func Configure() *gin.Engine {
	gin.SetMode("release")

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(cors())

	api := router.Group("/api")

	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		log.WithFields(log.Fields{
			"method":  param.Method,
			"client":  param.ClientIP,
			"latency": param.Latency,
		}).Debugf("[%d] %s %s", param.StatusCode, param.Method, param.Path)
		return ""
	}))

	api.GET("/ws", getGlobalWs)
	template := api.Group("/templates")
	{
		template.GET("/", getTemplates)
		template.GET("/:id", getTemplate)
		template.POST("/add", addTemplate)
	}

	servers := api.Group("/servers")
	{
		servers.GET("/", getServers)

		required := servers.Group("/:server", middleware.ServerRequired())
		required.GET("/ws", getServerWs)

		required.GET("/stats", getServerStats)
		required.GET("/files", getFiles)
		required.GET("/files/content", getFileContent)

		required.POST("/files", saveFileContent)
	}

	return router
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}
