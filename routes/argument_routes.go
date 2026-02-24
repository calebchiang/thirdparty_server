package routes

import (
	"github.com/calebchiang/thirdparty_server/controllers"
	"github.com/calebchiang/thirdparty_server/middleware"
	"github.com/gin-gonic/gin"
)

func ArgumentRoutes(r *gin.Engine) {
	auth := r.Group("/arguments")
	auth.Use(middleware.RequireAuth())
	{
		auth.GET("/", controllers.GetArguments)
		auth.POST("/", controllers.CreateArgument)
	}
}
