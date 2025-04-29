package router

import (
	"net/http"
	"fmt"
	"context"
	"log"
	"github.com/docker/docker/client"

	"github.com/Aadithya-J/alcaIDE/internal/docker"
	"github.com/Aadithya-J/alcaIDE/internal/handler"
)

func Setup(dockerManager *docker.DockerManager) http.Handler {

	containers := dockerManager.GetContainers()
    if len(containers) == 0 {
        log.Fatal("No containers available to serve requests.")
    }

	for _, c := range containers {
		fmt.Printf("Container ID: %s\n", c.ID)
	}

	ctx := context.Background()
	cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", handler.PingHandler)
	mux.HandleFunc("/register", handler.RegisterHandler)
	mux.HandleFunc("/login", handler.LoginHandler)

	mux.HandleFunc("/execPy", func(w http.ResponseWriter, r *http.Request) {
		handler.ExecPythonHandler(w, r, cli, ctx, containers)
	})
	return mux
}
