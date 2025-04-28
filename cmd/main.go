package main

import (
	"log"
	"net/http"
	"context"
	"io"
	"os"
	"syscall"
	"sync"
	"time"
	"os/signal"
	"github.com/Aadithya-J/alcaIDE/internal/router"
	"github.com/Aadithya-J/alcaIDE/internal/config"
	"github.com/Aadithya-J/alcaIDE/internal/db"
	"github.com/Aadithya-J/alcaIDE/model"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	
)

var containers []model.ContainerInfo

const INITIAL_CONTAINER_COUNT = 3

func main() {
	config.LoadEnv();

	db.Initialize();
	
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
        log.Fatalf("Failed to create Docker client: %v", err)
    }


	log.Println("Pulling Python image...")
	reader, err := cli.ImagePull(ctx, "docker.io/library/python:3.11-slim", image.PullOptions{})
	if err != nil {
		panic(err)
	} else {
		io.Copy(os.Stdout, reader)
		reader.Close()
	}


	log.Println("Creating and starting containers...")

	startedContainers := []model.ContainerInfo{}

	for i := 0; i < INITIAL_CONTAINER_COUNT; i++ {
        resp, err := cli.ContainerCreate(ctx, &container.Config{
            Image: "python:3.11-slim",
            Cmd:   []string{"sleep", "infinity"},
            Tty:   false,
        }, nil, nil, nil, "")

        if err != nil {
            log.Printf("Warning: Failed to create container %d: %v", i, err)
            continue
        }

        err = cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
        if err != nil {
            log.Printf("Warning: Failed to start container %s: %v", resp.ID, err)
            rmErr := cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{})
            if rmErr != nil {
                log.Printf("Warning: Failed to remove unstartable container %s: %v", resp.ID, rmErr)
            }
            continue
        }
        log.Printf("Started container %d: %s", i, resp.ID)
        startedContainers = append(startedContainers, model.ContainerInfo{ID: resp.ID})
    }

	containers = startedContainers

	if(len(containers) == 0) {
		log.Fatal("No containers started successfully.")
	}
	log.Println("Containers started successfully.")

	mux := router.Setup(containers)


	// --- Shutdown Setup ---
    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)


	go func() {
		log.Println("Server is running on :8080")
		err = server.ListenAndServe()
		if err != nil {
			log.Print(err)
		}
	}()

	sig := <-sigs
	log.Printf("Received signal: %s. Shutting down gracefully...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server exited gracefully")
	}

	log.Println("Removing containers...")
	cleanupCtx,cancelCleanup := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelCleanup()

	var wg sync.WaitGroup

	for _, c := range containers {
		wg.Add(1)
		go func(containerID string) {
			defer wg.Done()
			log.Printf("Stopping container %s...", containerID)

			stopTimeoutSeconds := 5
			if err := cli.ContainerStop(cleanupCtx, containerID, container.StopOptions{Timeout: &stopTimeoutSeconds}); err != nil {
				log.Printf("Error stopping container %s: %v", containerID, err)
			} else {
				log.Printf("Container %s stopped successfully", containerID)
			}

			log.Printf("Removing container %s...", containerID)

			if err := cli.ContainerRemove(cleanupCtx, containerID, container.RemoveOptions{}); err != nil {
				log.Printf("Error removing container %s: %v", containerID, err)
			} else {
				log.Printf("Container %s removed successfully", containerID)
			}
		}(c.ID)
	}
	wg.Wait()
	log.Println("All containers removed successfully.")

	log.Println("Closing database connection...")
	db.Close()

	log.Println("Closing Docker Client...")
	if err := cli.Close(); err != nil {
		log.Printf("Error closing Docker client: %v", err)
	} else {
		log.Println("Docker client closed successfully.")
	}
	log.Println("ShutDown application.")
}
