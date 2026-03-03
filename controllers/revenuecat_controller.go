package controllers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RevenueCatWebhook(c *gin.Context) {

	// Read raw request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Println("Failed to read RevenueCat webhook body:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	// Log the raw payload
	fmt.Println("REVENUECAT WEBHOOK RECEIVED")
	fmt.Println(string(body))

	// Respond OK so RevenueCat knows webhook succeeded
	c.JSON(http.StatusOK, gin.H{
		"status": "received",
	})
}
