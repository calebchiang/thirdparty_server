package controllers

import (
	"net/http"

	"github.com/calebchiang/thirdparty_server/database"
	"github.com/calebchiang/thirdparty_server/models"
	"github.com/calebchiang/thirdparty_server/services"
	"github.com/gin-gonic/gin"
)

func GetArguments(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var arguments []models.Argument

	if err := database.DB.
		Where("user_id = ?", userID.(uint)).
		Order("created_at desc").
		Find(&arguments).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch arguments"})
		return
	}

	c.JSON(http.StatusOK, arguments)
}

func CreateArgument(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse form fields
	personAName := c.PostForm("person_a_name")
	personBName := c.PostForm("person_b_name")
	persona := c.PostForm("persona")

	if personAName == "" || personBName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Person names are required"})
		return
	}

	// Validate persona
	validPersonas := map[string]bool{
		"mediator": true,
		"judge":    true,
		"comedic":  true,
	}

	if !validPersonas[persona] {
		persona = "mediator"
	}

	// Get audio file
	fileHeader, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Audio file is required"})
		return
	}

	// Generate transcript
	transcriptionResult, err := services.GenerateTranscript(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate transcript"})
		return
	}

	// Create argument
	argument := models.Argument{
		UserID:        userID.(uint),
		PersonAName:   personAName,
		PersonBName:   personBName,
		Persona:       persona,
		Transcription: transcriptionResult.Text,
		Status:        "processing",
	}

	if err := database.DB.Create(&argument).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create argument"})
		return
	}

	go services.ProcessJudgment(argument.ID)

	// Return immediately
	c.JSON(http.StatusCreated, gin.H{
		"id":            argument.ID,
		"user_id":       argument.UserID,
		"person_a_name": argument.PersonAName,
		"person_b_name": argument.PersonBName,
		"persona":       argument.Persona,
		"status":        argument.Status,
		"created_at":    argument.CreatedAt,
	})
}
