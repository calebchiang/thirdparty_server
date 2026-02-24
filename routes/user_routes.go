package routes

import (
	"github.com/calebchiang/thirdparty_server/controllers"
	"github.com/calebchiang/thirdparty_server/middleware"
	"github.com/gin-gonic/gin"
)

func UserRoutes(r *gin.Engine) {
	r.POST("/users", controllers.CreateUser)
	r.POST("/login", controllers.LoginUser)
	r.POST("/apple_login", controllers.AppleLogin)

	auth := r.Group("/users")
	auth.Use(middleware.RequireAuth())
	{
		auth.GET("/me", controllers.GetCurrentUser)
	}
}
