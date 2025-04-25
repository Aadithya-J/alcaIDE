package main

import (
	"log"
	"net/http"
	"context"
	"github.com/Aadithya-J/alcaIDE/internal/router"
	"github.com/jackc/pgx/v5"
	"os"
	"github.com/joho/godotenv"
)

func main() {
	//.env
	err := godotenv.Load()
    if err != nil {
        log.Println("Error loading .env file")
    }

	//db
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())
	log.Println("Connected toPostgres  Database!")


	//api
	mux := router.Setup()

	log.Println("Server is running on :8080")
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
