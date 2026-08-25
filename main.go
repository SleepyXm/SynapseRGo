package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"Synapse/agents"
	tokens "Synapse/handlers/tokens"
	"Synapse/rag"
	"Synapse/routes"
	"Synapse/storage"
	"Synapse/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("pgx", utils.Cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("DB not reachable:", err)
	}

	log.Println("DB connected")

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
}

func main() {
	utils.Load()
	utils.InitResend()
	utils.InitRedis()
	initDB()

	allowedOrigins := []string{utils.Cfg.DevServer}
	if utils.Cfg.FrontendProd != "" {
		allowedOrigins = append(allowedOrigins, utils.Cfg.FrontendProd)
	}

	router := gin.Default()
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Fatal(err)
	}

	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	api := router.Group("/api")
	objects, err := storage.NewLocalStore(utils.Cfg.RAGObjectRoot)
	if err != nil {
		log.Fatal("Failed to initialize local knowledge storage:", err)
	}
	knowledgeRepository := rag.NewPostgresRepository(db)
	tokenResolver := func(_ context.Context, userID, tokenName string) (string, error) {
		return tokens.GetDecryptedToken(db, userID, tokenName)
	}
	search := rag.NewSearchService(knowledgeRepository, tokenResolver)
	agentRepository := agents.NewPostgresRepository(db)
	registry := agents.NewRegistry()
	runtime := agents.NewRuntime(agentRepository, search, tokenResolver, registry)
	worker := rag.NewWorker(
		knowledgeRepository,
		objects,
		tokenResolver,
		time.Duration(utils.Cfg.RAGWorkerPollSeconds)*time.Second,
	)
	go worker.Run(context.Background())

	routes.RegisterAuthRoutes(api.Group("/auth"), db)
	routes.RegisterLLMRoutes(api.Group("/llm"), db, runtime, knowledgeRepository, registry)
	routes.RegisterConversationRoutes(api.Group("/conversation"), db)
	routes.RegisterTokenRoutes(api.Group("/tokens"), db)
	routes.RegisterFavoriteRoutes(api.Group("/user"), db)
	routes.RegisterKnowledgeRoutes(api.Group("/knowledge-bases"), db, knowledgeRepository, search, objects)
	routes.RegisterAgentRoutes(api, db, agentRepository, knowledgeRepository, runtime, registry)

	router.Run(":8000")
}
