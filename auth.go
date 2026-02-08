package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

func CheckAuthHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("auth_user_session")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Return the username so the frontend can say "Welcome, szinn!"
	json.NewEncoder(w).Encode(map[string]string{"username": cookie.Value})
}

func SignupHandler(w http.ResponseWriter, r *http.Request) {
	emails := os.Getenv("EMAILS")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	hashedPswd, err := bcrypt.GenerateFromPassword([]byte(u.Password), 10)
	if err != nil {
		http.Error(w, "Error processing password", http.StatusInternalServerError)
		return
	}

	email := u.Email
	if !strings.Contains(emails, email) {
		http.Error(w, "Cannot accept any new users", http.StatusConflict)
		return
	}

	query := "INSERT INTO user (username, password, email) VALUES (?, ?, ?)"
	_, err = DB.Exec(query, u.Username, hashedPswd, email)
	if err != nil {
		fmt.Println("DB Error: ", err)
		http.Error(w, "Username already exists or db error", http.StatusConflict)
		return
	}

	// Make folder in dir
	if err := os.Mkdir(("./root/" + u.Username), 0755); err != nil {
		log.Println("Error creating folder for new user: " + u.Username)
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User created successfully!"))
	return
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var storedHash string
	query := "SELECT password FROM user WHERE username = ? LIMIT 1"
	err := DB.QueryRow(query, u.Username).Scan(&storedHash)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Incorrect username/password combo", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(u.Password))
	if err != nil {
		http.Error(w, "Incorrect username/password combo", http.StatusUnauthorized)
		return
	}

	cookie := &http.Cookie{
		Name:     "auth_user_session",
		Value:    u.Username,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600,
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("User found"))
	return
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     "auth_user_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
	}

	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logged out successfully"))
}
