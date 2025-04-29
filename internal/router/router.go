package router

import (
	"net/http"

	"github.com/Aadithya-J/alcaIDE/internal/docker"
	"github.com/Aadithya-J/alcaIDE/internal/handler"
)

func Setup(dockerManager *docker.DockerManager) http.Handler {
	// containers := dockerManager.GetContainers()
	// for _, c := range containers {
	// 	fmt.Printf("Container ID: %s\n", c.ID)
	// }

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", handler.PingHandler)
	mux.HandleFunc("/register", handler.RegisterHandler)
	mux.HandleFunc("/login", handler.LoginHandler)

	mux.HandleFunc("/exec", func(w http.ResponseWriter, r *http.Request) {
		handler.ExecCodeHandler(w, r, r.Context(), dockerManager)
	})
	return mux
}
