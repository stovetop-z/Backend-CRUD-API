package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func main() {
	// Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found")
	}

	// Initialize the db
	err, msg := InitDB()
	fmt.Println(msg)

	// A simple health check
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Pong! bunchazinns.org is live.")
	})

	fileServer := http.FileServer(http.Dir("./uploads"))
	http.Handle("/media/", http.StripPrefix("/media/", fileServer))

	// 4. Start the Server
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("PORT") // Default port if not specified
	}

	fmt.Printf("Server starting on port %s...\n", port)

	// Access signup
	http.HandleFunc("/signup", SignupHandler)

	// Access login
	http.HandleFunc("/login", LoginHandler)

	// Handle upload
	http.HandleFunc("/upload", UploadHandler)

	// This line "blocks" and keeps the program running
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
