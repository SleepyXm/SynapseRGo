package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/SleepyXm/SynapseRGo/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("pgx", os.Getenv("DATABASE"))
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("DB not reachable:", err)
	}

	log.Println("DB connected")
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found", err)
	}

	initDB()

	var jwtSecret []byte

	allowedOrigins := []string{}
	if dev := os.Getenv("DEV_SERVER"); dev != "" {
		allowedOrigins = append(allowedOrigins, dev)
	}
	if prod := os.Getenv("FRONTEND_PROD"); prod != "" {
		allowedOrigins = append(allowedOrigins, prod)
	}
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000"}
	}

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	auth := router.Group("/auth")
	routes.RegisterAuthRoutes(auth, db, jwtSecret)
	llm := router.Group("/llm")
	routes.RegisterLLMRoutes(llm, db, jwtSecret)
	conversations := router.Group("/conversation")
	routes.RegisterConversationRoutes(conversations, db, jwtSecret)
	tokens := router.Group("/tokens")
	routes.RegisterTokenRoutes(tokens, db, jwtSecret)

	router.Run(":8000")
}
