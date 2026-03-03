package routes

import (
	"github.com/calebchiang/thirdparty_server/controllers"
	"github.com/gin-gonic/gin"
)

func RevenueCatRoutes(r *gin.Engine) {
	r.POST("/revenuecat/webhook", controllers.RevenueCatWebhook)
}
