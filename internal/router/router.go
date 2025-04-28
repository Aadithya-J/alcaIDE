package router

import (
	"net/http"
	"fmt"
	// "context"
	"github.com/Aadithya-J/alcaIDE/internal/handler"
	"github.com/Aadithya-J/alcaIDE/model"
)

func Setup(containerIDs []model.ContainerInfo) http.Handler {
	for _, c := range containerIDs {
		fmt.Printf("Container ID: %s\n", c.ID)
	}

	//ctx := context.Background()
	//cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", handler.PingHandler)
	mux.HandleFunc("/register", handler.RegisterHandler)
	mux.HandleFunc("/login", handler.LoginHandler)
	return mux
}
