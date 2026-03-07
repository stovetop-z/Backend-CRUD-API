package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Replace with your specific frontend URL for production
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight 'OPTIONS' requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found")
	}

	// Initialize the db
	err, msg := InitDB()
	fmt.Println(msg)

	// Server mux
	mux := http.NewServeMux()

	// health check
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Pong! bunchazinns.org is live.")
	})

	mux.HandleFunc("/signup", SignupHandler)
	mux.HandleFunc("/login", LoginHandler)
	mux.HandleFunc("/logout", LogoutHandler)

	// Protected Routes (using your AuthMiddleware)
	mux.HandleFunc("/check-auth", AuthMiddleware(CheckAuthHandler))
	mux.HandleFunc("/upload", AuthMiddleware(UploadHandler))
	mux.HandleFunc("/delete", AuthMiddleware(DeleteHandler))
	mux.HandleFunc("/photos", AuthMiddleware(GetPhotosHandler))
	mux.HandleFunc("/search", AuthMiddleware(SearchPhotosHandler))

	// Static Media Serving
	mediaPath := os.Getenv("MEDIA_PATH")

	fmt.Println("Check: Looking for images in:", mediaPath)

	fileServer := http.FileServer(http.Dir(mediaPath))

	// This maps http://localhost:8080/media/ to that folder
	mux.Handle("/media/", http.StripPrefix("/media/", fileServer))

	// Start the Server
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("PORT") // Default port if not specified
	}

	fmt.Printf("Server starting on port %s...\n", port)

	log.Fatal(http.ListenAndServe(":"+port, CORSMiddleware(mux)))
}
