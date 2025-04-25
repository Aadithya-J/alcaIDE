package db

import (
    "context"
    "log"
    "os"
    
    "github.com/jackc/pgx/v5"
)

var Conn *pgx.Conn

func Initialize() {
    var err error
    Conn, err = pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatalf("Unable to connect to database: %v", err)
        os.Exit(1)
    }
    log.Println("Connected to Postgres Database!")
}

func Close() {
    if Conn != nil {
        Conn.Close(context.Background())
        log.Println("Database connection closed.")
    }
}