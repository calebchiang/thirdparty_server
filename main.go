package main

import (
	"github.com/calebchiang/thirdparty_server/database"
	"github.com/calebchiang/thirdparty_server/models"
	"github.com/calebchiang/thirdparty_server/routes"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	database.Connect()
	database.DB.AutoMigrate(
		&models.User{},
		&models.Argument{},
		&models.Judgment{},
	)

	r := gin.Default()
	routes.UserRoutes(r)
	routes.ArgumentRoutes(r)

	r.Run()
}
