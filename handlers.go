package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

var photoUploadPath string = "./images/"

type photoDelete struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check cookie auth
	cookie, err := r.Cookie("auth_user_session")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			http.Error(w, "Unauthorized: Please login", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	fmt.Printf("Cookie found: %v\n", cookie.Name)

	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "Could not get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Extract metadata date and time
	var takenTime time.Time
	x, exifErr := exif.Decode(file)
	if exifErr == nil {
		if tm, tmErr := x.DateTime(); tmErr == nil {
			takenTime = tm
		}
	}

	// Fallback to now if metadata is missing
	if takenTime.IsZero() {
		takenTime = time.Now()
	}

	// Reset file pointer to the beginning after reading
	file.Seek(0, 0)

	dateStr := takenTime.Format("2006-01-02")
	timeStr := takenTime.Format("15:04:05")

	nameParts := strings.Split(header.Filename, ".")
	name := nameParts[0]
	ext := ""
	if len(nameParts) > 1 {
		ext = nameParts[len(nameParts)-1]
	}

	// Add timestamp to filename to prevent overwrites
	uniqueName := fmt.Sprintf("%d_%s.%s", time.Now().Unix(), name, ext)
	savePath := photoUploadPath + uniqueName

	// Save to Disk
	dst, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "Failed to create file on server", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save image bits", http.StatusInternalServerError)
		return
	}

	userID := QueryID(cookie.Value)
	query := "INSERT INTO photo (user_id, date, time, path, name, ext) VALUES (?, ?, ?, ?, ?, ?)"
	_, err = DB.Exec(query, userID, dateStr, timeStr, savePath, name, ext)
	if err != nil {
		fmt.Printf("SQL Error: %v\n", err)
		http.Error(w, "Database save failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Successfully uploaded: %s", uniqueName)
	return
}

func DeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Let's get the frontend submission data, should just be the id number of the photo
	var photoInfo photoDelete
	if err := json.NewDecoder(r.Body).Decode(&photoInfo); err != nil {
		http.Error(w, "Invalid photo", http.StatusBadRequest)
		return
	}

	var query string
	if id := photoInfo.ID; len(id) > 0 {
		query = ""
	}

}
