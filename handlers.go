package main

/*
#cgo CXXFLAGS: -I./build/_deps/exiv2-src/include -I./build -std=c++17
#cgo LDFLAGS: -L./build/lib -Wl,-rpath,./build/lib -lexiv2 -lexpat -lz -lpthread -lstdc++
#include <stdlib.h>

char* getMetaData(const char* path);
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"google.golang.org/genai"
)

// Get the image datetime function
func GetImageDateTime(path string) string {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// Call the C function
	cResult := C.getMetaData(cPath)
	if cResult == nil {
		return ""
	}

	defer C.free(unsafe.Pointer(cResult))
	return C.GoString(cResult)
}

type photoResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Date string `json:"date"`
	Time string `json:"time"`
}

type photoDelete struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func HasPhotoFolder(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), err
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	// API_KEY := os.Getenv("API_KEY")
	photoUploadPath := os.Getenv("IMG_PATH")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	/*******************************************
		Check the cookies for authorization to
		upload
	********************************************/
	cookie, err := r.Cookie("auth_user_session")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			http.Error(w, "Unauthorized: Please login", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Cookie found!\n")

	/*******************************************
		Get the file from the frontend and
		extract the metadata
	********************************************/
	file, header, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "Could not get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Prevent path traversal by cleaning the username
	userBaseDir := strings.Replace(photoUploadPath, "USER", cookie.Value, 1)

	if exists, _ := HasPhotoFolder(userBaseDir); !exists {
		os.MkdirAll(userBaseDir, 0755)
	}

	// Generate unique filename
	lastDot := strings.LastIndex(header.Filename, ".")
	name := header.Filename
	ext := "jpg"
	if lastDot != -1 {
		name = header.Filename[:lastDot]
		ext = header.Filename[lastDot+1:]
	}

	uniqueName := fmt.Sprintf("%d_%s.%s", time.Now().Unix(), name, ext)
	fullPath := filepath.Join(userBaseDir, uniqueName)

	// Save to Disk
	dst, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, "Internal save error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}

	// Upload to google gemini to create keywords
	ctx := context.Background()
	apiKey := os.Getenv("API_KEY")
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Println(err)
	}

	imageBytes, err := os.ReadFile(fullPath)
	if err != nil {
		log.Println(err)
	}

	prompt := "This prompt is for an api. Create 10 relevant keywords for this image separated only by commas by looking at what is in the image (people, objects, location, etc). Do not respond with any other words except for the keywords separated by commas. Here is an example output I expect: keyword1, keyword2, ... keyword10"

	mimeType := "image/" + ext
	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{Data: imageBytes, MIMEType: mimeType}},
	}

	result, err := client.Models.GenerateContent(ctx, "gemini-2.5-flash", []*genai.Content{{Parts: parts}}, nil)

	if err != nil {
		log.Println("Gemini error:", err)
	}

	keywords := result.Candidates[0].Content.Parts[0].Text
	if err != nil {
		log.Println("Error getting the text")
	}

	// Metadata Extraction
	dt := GetImageDateTime(fullPath)
	fmt.Println("Date Time extraction:", dt)

	var dateStr, timeStr string

	// If we have a valid EXIF string (expected format "YYYY:MM:DD HH:MM:SS")
	if dt != "" && strings.Contains(dt, " ") {
		parts := strings.SplitN(dt, " ", 2)
		dateStr = strings.ReplaceAll(parts[0], ":", "-")
		timeStr = parts[1]
	} else {
		// Fallback to current system time if metadata is missing or malformed
		now := time.Now()
		dateStr = now.Format("2006-01-02")
		timeStr = now.Format("15:04:05")
	}

	// DB Entry
	userID := QueryID(cookie.Value)
	query := "INSERT INTO photo (user_id, date, time, path, name, ext) VALUES (?, ?, ?, ?, ?, ?)"
	_, err = DB.Exec(query, userID, dateStr, timeStr, fullPath, name, ext)
	if err != nil {
		os.Remove(fullPath) // Cleanup file if DB fails
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var photo_id int
	if err := DB.QueryRow("SELECT id FROM photo WHERE name = ?", name).Scan(&photo_id); err != nil {
		log.Println("Failed to acquire photo_id with name")
	}

	if err := KeywordsHandler(keywords, photo_id); err != nil {
		log.Println(err)
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Uploaded: %s", uniqueName)
	return
}

func DeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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

	// Get the user signed in, making the request of deletion
	userID := QueryID(cookie.Value)

	// Let's get the frontend submission data, should just be the id number of the photo
	var photoInfo photoDelete
	if err := json.NewDecoder(r.Body).Decode(&photoInfo); err != nil {
		http.Error(w, "Invalid photo", http.StatusBadRequest)
		return
	}

	// First, get path then delete from the Database
	query := "SELECT path FROM photo WHERE id = ? AND user_id = ?"
	queryDelete := "DELETE FROM photo WHERE id = ? AND user_id = ?"
	var path string

	name := photoInfo.Name
	id := photoInfo.ID
	if name == "" || id == "" {
		http.Error(w, "Name or ID not provided", http.StatusBadRequest)
		return
	}

	if err := DB.QueryRow(query, id, userID).Scan(&path); err != nil {
		http.Error(w, "Database error in retreiving path and ext of image", http.StatusInternalServerError)
		return
	}
	if _, err := DB.Exec(queryDelete, id, userID); err != nil {
		http.Error(w, "Photo not found in db or incorrect information given", http.StatusNotFound)
		return
	}

	// Now, delete from folder
	if err := os.Remove(path); err != nil {
		http.Error(w, "Internal error trying to delete image", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Successfully deleted image"))
	return
}

func GetPhotosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
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

	rows, err := DB.Query("SELECT id, name, path, date, time FROM photo WHERE user_id = ? ORDER BY date DESC, time DESC", QueryID(cookie.Value))
	if err != nil {
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var photos []photoResponse
	for rows.Next() {
		var p photoResponse
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Date, &p.Time); err != nil {
			continue
		}

		// Transform absolute path to web-accessible path
		if strings.Contains(p.Path, "root/") {
			p.Path = "/media/" + p.Path[strings.Index(p.Path, "root/")+5:]
		}

		photos = append(photos, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}

func KeywordsHandler(keywords string, photo_id int) error {
	kw := strings.Split(keywords, ", ")

	// Insert keywords (ignoring duplicates)
	// Using "OR IGNORE" for SQLite or "ON CONFLICT DO NOTHING" for Postgres
	insQuery := "INSERT IGNORE INTO keyword (word) VALUES (?)"
	for _, k := range kw {
		if _, err := DB.Exec(insQuery, k); err != nil {
			log.Printf("Couldn't insert keyword %s: %v", k, err)
		}
	}

	// Get the IDs and Link to Photo
	// We can combine these to avoid extra loops
	linkQuery := "INSERT INTO photo_keyword (photo_id, keyword_id) VALUES (?, (SELECT id FROM keyword WHERE word = ?))"
	for _, k := range kw {
		if _, err := DB.Exec(linkQuery, photo_id, k); err != nil {
			// Check if link already exists to avoid errors on re-tagging
			log.Printf("Link already exists or error for %s: %v", k, err)
		}
	}

	return nil
}

func SearchPhotosHandler(w http.ResponseWriter, r *http.Request) {
	queryParam := r.URL.Query().Get("q")
	if queryParam == "" {
		GetPhotosHandler(w, r) // Fallback to all photos if search is empty
		return
	}

	cookie, _ := r.Cookie("auth_user_session")
	userID := QueryID(cookie.Value)

	// JOIN query to find photos linked to specific keywords
	rows, err := DB.Query(`
        SELECT DISTINCT p.id, p.name, p.path, p.date, p.time 
        FROM photo p
        JOIN photo_keyword pk ON p.id = pk.photo_id
        JOIN keyword k ON pk.keyword_id = k.id
        WHERE p.user_id = ? AND k.word LIKE ?`,
		userID, "%"+queryParam+"%")

	if err != nil {
		http.Error(w, "Search error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var photos []photoResponse
	for rows.Next() {
		var p photoResponse
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Date, &p.Time); err != nil {
			continue
		}
		p.Path = "/media/" + strings.Split(p.Path, "/root/")[1]
		photos = append(photos, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}
