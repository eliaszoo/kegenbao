package main

import (
	"fmt"
	"log"
	"path/filepath"

	"kegenbao/internal/config"
	"kegenbao/internal/database"
	"kegenbao/internal/router"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("Loaded config: port=%s, env=%s", cfg.Server.Port, cfg.Server.Env)

	// Initialize database
	if err := database.InitDB(&cfg.Database); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Get frontend path
	frontendPath := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(cfg.Database.Path)))), "frontend")

	// Setup router
	r := router.SetupRouter(frontendPath)

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Server starting on %s", addr)
	log.Printf("Frontend path: %s", frontendPath)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}