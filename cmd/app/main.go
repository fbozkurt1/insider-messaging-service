package main

import (
	"context"
	"fmt"
	"insider-message-service/config"
	"insider-message-service/internal/api"
	"insider-message-service/internal/cache"
	"insider-message-service/internal/repository"
	"insider-message-service/internal/services"
	"insider-message-service/internal/worker"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// @title           Messaging Service
// @version         1.0
// @description     Sends and receives messages

// @host      localhost:8080
// @BasePath  /api
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	cfg := config.LoadConfig()
	dbPool, redisClient, err := setupDependencies(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to setup dependencies: %v", err)
	}
	defer dbPool.Close()
	defer redisClient.Close()

	jobManager, server := buildApplication(dbPool, redisClient, &wg, ctx, cfg)

	startBackgroundJob(&wg, jobManager, ctx)
	startServer(server)

	waitForShutdown(server, cancel, &wg)

	log.Println("Server gracefully stopped")
}

func setupDependencies(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, *redis.Client, error) {
	dbPool, err := repository.NewConnection(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to establish database connection: %w", err)
	}
	log.Println("Database connection established.")

	redisClient, err := cache.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to establish Redis connection: %w", err)
	}
	log.Println("Redis connection established.")

	return dbPool, redisClient, nil
}

func buildApplication(dbPool *pgxpool.Pool, redisClient *redis.Client, wg *sync.WaitGroup, appCtx context.Context, cfg *config.Config) (*worker.JobManager, *http.Server) {
	messageRepository := repository.NewMessageRepository(dbPool)
	messageCache := cache.NewMessageCache(redisClient)

	messageService := services.NewMessageService(messageRepository, messageCache)
	jobManager := worker.NewJobManager(messageService, wg)
	apiHandler := api.NewHandler(messageService, jobManager, appCtx)

	router := api.NewRouter(apiHandler)
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	log.Println("Application components built successfully.")
	return jobManager, server
}

func startBackgroundJob(wg *sync.WaitGroup, jobManager *worker.JobManager, ctx context.Context) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := jobManager.Start(ctx); err != nil {
			log.Printf("Unexpected error while starting job: %v", err)
		}
	}()
	log.Println("Background job started.")
}

func startServer(server *http.Server) {
	go func() {
		log.Printf("HTTP server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Unexpected error while starting server: %v", err)
		}
	}()
}

func waitForShutdown(server *http.Server, cancelApp context.CancelFunc, wg *sync.WaitGroup) {
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-shutdownChan

	log.Println("Shutting down gracefully...")

	// wait HTTP server 15 seconds to shut down
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Unexpected error while shutting down server: %v", err)
	}

	cancelApp()
	wg.Wait()
}
