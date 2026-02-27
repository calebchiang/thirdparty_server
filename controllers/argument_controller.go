package controllers

import (
	"net/http"
	"os"

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
		Preload("Judgment").
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

	// Get uploaded file
	fileHeader, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Audio file is required"})
		return
	}

	// Enforce file size limit (50MB)
	const maxFileSize = 50 << 20 // 50MB
	if fileHeader.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 50MB)"})
		return
	}

	// Normalize media (handles video + audio formats)
	mediaService := services.NewMediaService()

	normalizedPath, err := mediaService.Normalize(fileHeader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process media"})
		return
	}

	// Generate transcript from normalized file
	transcriptionResult, err := services.GenerateTranscriptFromPath(normalizedPath)
	if err != nil {
		_ = os.Remove(normalizedPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate transcript"})
		return
	}

	// Clean up normalized file after transcription
	_ = os.Remove(normalizedPath)

	// Create argument record
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

	// Run judgment asynchronously
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

func GetArgumentByID(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")

	var argument models.Argument

	if err := database.DB.
		Preload("Judgment").
		Where("id = ? AND user_id = ?", id, userID.(uint)).
		First(&argument).Error; err != nil {

		c.JSON(http.StatusNotFound, gin.H{"error": "Argument not found"})
		return
	}

	c.JSON(http.StatusOK, argument)
}

func DeleteArgument(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")

	var argument models.Argument

	// Ensure argument exists AND belongs to user
	if err := database.DB.
		Where("id = ? AND user_id = ?", id, userID.(uint)).
		First(&argument).Error; err != nil {

		c.JSON(http.StatusNotFound, gin.H{"error": "Argument not found"})
		return
	}

	// Delete (Judgment will cascade automatically)
	if err := database.DB.Delete(&argument).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete argument"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Argument deleted successfully",
	})
}

func CreateArgumentByScreenshot(c *gin.Context) {
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

	// Parse multiple images (frontend should send: screenshots[])
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Screenshots are required"})
		return
	}

	files := form.File["screenshots"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one screenshot is required"})
		return
	}

	// Optional: enforce per-file size limit (10MB each)
	const maxFileSize = 10 << 20
	for _, file := range files {
		if file.Size > maxFileSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Each screenshot must be under 10MB"})
			return
		}
	}

	// Create argument record FIRST (status = processing)
	argument := models.Argument{
		UserID:        userID.(uint),
		PersonAName:   personAName,
		PersonBName:   personBName,
		Persona:       persona,
		Transcription: "", // not used for screenshots
		Status:        "processing",
	}

	if err := database.DB.Create(&argument).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create argument"})
		return
	}

	// Call screenshot judgment service (we implement next)
	result, err := services.GenerateScreenshotJudgment(
		personAName,
		personBName,
		persona,
		files,
	)

	if err != nil {
		database.DB.Model(&argument).Update("status", "failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate judgment"})
		return
	}

	// Save judgment immediately
	judgment := models.Judgment{
		ArgumentID:              argument.ID,
		Winner:                  result.Winner,
		Reasoning:               result.Reasoning,
		FullResponse:            result.FullResponse,
		Respect:                 result.Respect,
		Empathy:                 result.Empathy,
		Accountability:          result.Accountability,
		EmotionalRegulation:     result.EmotionalRegulation,
		ManipulationToxicity:    result.ManipulationToxicity,
		ConversationHealthScore: result.ConversationHealthScore,
	}

	if err := database.DB.Create(&judgment).Error; err != nil {
		database.DB.Model(&argument).Update("status", "failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save judgment"})
		return
	}

	database.DB.Model(&argument).Update("status", "complete")

	// Return full argument with judgment
	c.JSON(http.StatusCreated, gin.H{
		"id":            argument.ID,
		"user_id":       argument.UserID,
		"person_a_name": argument.PersonAName,
		"person_b_name": argument.PersonBName,
		"persona":       argument.Persona,
		"status":        "complete",
		"judgment":      judgment,
		"created_at":    argument.CreatedAt,
	})
}
