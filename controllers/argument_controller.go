package controllers

import (
	"net/http"

	"github.com/calebchiang/thirdparty_server/database"
	"github.com/calebchiang/thirdparty_server/models"
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

	var input struct {
		PersonAName   string `json:"person_a_name"`
		PersonBName   string `json:"person_b_name"`
		Transcription string `json:"transcription"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if input.PersonAName == "" || input.PersonBName == "" || input.Transcription == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
		return
	}

	argument := models.Argument{
		UserID:        userID.(uint),
		PersonAName:   input.PersonAName,
		PersonBName:   input.PersonBName,
		Transcription: input.Transcription,
	}

	if err := database.DB.Create(&argument).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create argument"})
		return
	}

	c.JSON(http.StatusCreated, argument)
}
