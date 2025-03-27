package main

import (
	"pplx2api/config"
	"pplx2api/router"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	// Load configuration

	// Setup all routes
	router.SetupRoutes(r)

	// Run the server on 0.0.0.0:8080
	r.Run(config.ConfigInstance.Address)
}
