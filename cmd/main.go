package main

import (
	"log"
	"net/http"
	"github.com/Aadithya-J/alcaIDE/internal/router"
	"github.com/Aadithya-J/alcaIDE/internal/config"
	"github.com/Aadithya-J/alcaIDE/internal/db"
)

func main() {
	config.LoadEnv();

	db.Initialize();
	defer db.Close()

	mux := router.Setup()

	log.Println("Server is running on :8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
