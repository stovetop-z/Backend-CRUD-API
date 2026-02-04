package main

import (
	"database/sql"
	"os"
	"strings"
)

var DB *sql.DB

var dsn string = "USER:PASS@tcp(127.0.0.1:3306)/family_server"

func InitDB() (error, string) {
	DB_USER := os.Getenv("DB_USER")
	DB_PASS := os.Getenv("DB_PASSWORD")

	dsn = strings.Replace(dsn, "USER", DB_USER, 1)
	dsn = strings.Replace(dsn, "PASS", DB_PASS, 1)

	var err error
	DB, err = sql.Open("mysql", dsn)

	if err != nil {
		return err, "Yikes"
	}
	DB.Ping()

	return err, "Running"
}

func QueryID(username string) int {
	var userID int
	query := "SELECT id FROM user WHERE username = ? LIMIT 1"
	err := DB.QueryRow(query, username).Scan(&userID)
	if err != nil {
		return -1
	}

	return userID
}
