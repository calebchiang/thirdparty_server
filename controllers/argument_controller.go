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

	if personAName == "" || personBName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Person names are required"})
		return
	}

	// Get audio file
	fileHeader, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Audio file is required"})
		return
	}

	// Generate transcript via service
	transcriptionResult, err := services.GenerateTranscript(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate transcript"})
		return
	}

	// Create argument record
	argument := models.Argument{
		UserID:        userID.(uint),
		PersonAName:   personAName,
		PersonBName:   personBName,
		Transcription: transcriptionResult.Text,
	}

	if err := database.DB.Create(&argument).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create argument"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            argument.ID,
		"user_id":       argument.UserID,
		"person_a_name": argument.PersonAName,
		"person_b_name": argument.PersonBName,
		"transcription": argument.Transcription,
		"language":      transcriptionResult.Language,
		"duration":      transcriptionResult.Duration,
		"segments":      transcriptionResult.Segments,
		"created_at":    argument.CreatedAt,
	})
}
