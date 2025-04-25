package router

import (
	"net/http"

	"github.com/Aadithya-J/alcaIDE/internal/handler"
)

func Setup() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", handler.PingHandler)
	mux.HandleFunc("/register", handler.RegisterHandler)
	mux.HandleFunc("/login", handler.LoginHandler)
	return mux
}
