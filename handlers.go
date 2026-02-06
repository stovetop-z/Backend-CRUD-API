package main

/*
#cgo CXXFLAGS: -I./build/_deps/exiv2-src/include -I./build -std=c++17
#cgo LDFLAGS: -L./build/lib -Wl,-rpath,./build/lib -lexiv2 -lexpat -lz -lpthread -lstdc++
#include <stdlib.h>

char* getMetaData(const char* path);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
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

var photoUploadPath string = os.Getenv("IMG_UPLOAD")

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
	safeUser := filepath.Base(filepath.Clean(cookie.Value))
	userBaseDir := strings.Replace(photoUploadPath, "USER", safeUser, 1)

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

	// Metadata Extraction
	dt := GetImageDateTime(fullPath)
	dateStr := time.Now().Format("2006-01-02")
	timeStr := time.Now().Format("15:04:05")

	if dt != "" && strings.Contains(dt, "_") {
		parts := strings.Split(dt, "_")
		if len(parts) >= 2 {
			dateStr = parts[0]
			timeStr = parts[1]
		}
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
	fmt.Printf("Cookie found: %v\n", cookie.Name)

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

	rows, err := DB.Query("SELECT id, name, path, date, time FROM photo ORDER BY date DESC, time DESC")
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
		photos = append(photos, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}
