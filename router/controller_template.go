package router

import (
	"daemon/templates"
	"github.com/apex/log"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func getTemplates(c *gin.Context) {
	t, err := templates.GetTemplates()
	if err != nil {
		log.WithError(err).Error("Failed to get templates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get templates"})
		return
	}

	c.JSON(http.StatusOK, t)
}

func getTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.WithError(err).Error("Failed to convert id")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to convert id"})
		return
	}

	t, err := templates.GetTemplate(id)
	if err != nil {
		log.WithError(err).Error("Failed to get template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		return
	}

	c.JSON(http.StatusOK, t)
}

func addTemplate(c *gin.Context) {
	var t templates.Template
	if err := c.ShouldBindJSON(&t); err != nil {
		log.WithError(err).Error("Failed to bind template")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to bind template"})
		return
	}

	if err := templates.AddTemplate(t); err != nil {
		log.WithError(err).Error("Failed to import template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import template"})
		return
	}

	c.JSON(http.StatusOK, t)
}
