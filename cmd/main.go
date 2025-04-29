package main

import (
    "context"
    "errors"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/Aadithya-J/alcaIDE/internal/config"
    "github.com/Aadithya-J/alcaIDE/internal/db"
    "github.com/Aadithya-J/alcaIDE/internal/docker"
    "github.com/Aadithya-J/alcaIDE/internal/router"
)

const (
    INITIAL_CONTAINER_COUNT = 3
    SERVER_SHUTDOWN_TIMEOUT = 5 * time.Second
    SERVER_ADDR             = ":8080"
    DOCKER_IMAGE            = "docker.io/library/python:3.11-slim"
)

func main() {
    config.LoadEnv()

    db.Initialize()
    defer func() {
        log.Println("Closing database connection...")
        db.Close()
    }()

    dockerManager, err := docker.NewManager(DOCKER_IMAGE)
    if err != nil {
        log.Fatalf("Failed to create Docker manager: %v", err)
    }
    defer dockerManager.Close()

    ctx := context.Background()
    if err := dockerManager.PullImage(ctx); err != nil {
        log.Printf("Warning: Failed to pull Docker image: %v. Proceeding might use a local image.", err)
    }

    if err := dockerManager.StartInitialContainers(ctx, INITIAL_CONTAINER_COUNT); err != nil {
        log.Fatalf("Failed to start initial containers: %v", err)
    }

    defer dockerManager.CleanupContainers()

    mux := router.Setup(dockerManager)

    server := &http.Server{
        Addr:    SERVER_ADDR,
        Handler: mux,
    }

    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        log.Printf("Server starting on %s", SERVER_ADDR)
        if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatalf("Server ListenAndServe error: %v", err)
        }
    }()

    sig := <-sigs
    log.Printf("Received signal: %s. Shutting down gracefully...", sig)

    shutdownCtx, cancel := context.WithTimeout(context.Background(), SERVER_SHUTDOWN_TIMEOUT)
    defer cancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        log.Printf("Server forced to shutdown: %v", err)
    } else {
        log.Println("Server exited gracefully")
    }

    log.Println("Application shutdown complete.")
}
