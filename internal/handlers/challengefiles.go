package handlers

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/ajesus37/hCTF2/internal/database"
	"github.com/ajesus37/hCTF2/internal/storage"
)

type ChallengeFileHandler struct {
	db      *database.DB
	storage storage.Storage
}

func NewChallengeFileHandler(db *database.DB, storage storage.Storage) *ChallengeFileHandler {
	return &ChallengeFileHandler{db: db, storage: storage}
}

// UploadFile handles POST /api/admin/challenges/{id}/files
func (h *ChallengeFileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	challengeID := chi.URLParam(r, "id")

	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB
		http.Error(w, "File too large (max 50MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Upload to storage
	url, err := h.storage.Upload(r.Context(), header.Filename, file)
	if err != nil {
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	// Save to database
	sizeBytes := header.Size
	_, err = h.db.CreateChallengeFile(challengeID, header.Filename, "local", url, &sizeBytes)
	if err != nil {
		if delErr := h.storage.Delete(r.Context(), url); delErr != nil {
			log.Printf("warning: failed to clean up uploaded file %s after db error: %v", url, delErr)
		}
		http.Error(w, "Failed to save file record", http.StatusInternalServerError)
		return
	}

	// Return updated file list
	h.renderFileList(w, challengeID)
}

// AddExternalURL handles POST /api/admin/challenges/{id}/files/url
func (h *ChallengeFileHandler) AddExternalURL(w http.ResponseWriter, r *http.Request) {
	challengeID := chi.URLParam(r, "id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	externalURL := r.FormValue("external_url")
	if externalURL == "" {
		http.Error(w, "No URL provided", http.StatusBadRequest)
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		// Extract filename from URL
		filename = filepath.Base(externalURL)
		if filename == "" || filename == "/" {
			filename = "file"
		}
	}

	// Save to database
	_, err := h.db.CreateChallengeFile(challengeID, filename, "external", externalURL, nil)
	if err != nil {
		http.Error(w, "Failed to save file record", http.StatusInternalServerError)
		return
	}

	// Return updated file list
	h.renderFileList(w, challengeID)
}

// DeleteFile handles DELETE /api/admin/challenges/files/{file_id}
func (h *ChallengeFileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")

	// Get file info first
	file, err := h.db.GetChallengeFileByID(fileID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Delete from storage if local
	if file.StorageType == "local" {
		if err := h.storage.Delete(r.Context(), file.StoragePath); err != nil {
			log.Printf("warning: failed to delete file from storage %s: %v", file.StoragePath, err)
		}
	}

	// Delete from database
	if err := h.db.DeleteChallengeFile(fileID); err != nil {
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	// Return updated file list
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="text-gray-400 text-sm">File deleted.</div>`))
}

// ListFiles handles GET /api/admin/challenges/{id}/files
func (h *ChallengeFileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	challengeID := chi.URLParam(r, "id")
	h.renderFileList(w, challengeID)
}

// renderFileList renders the HTML for the file list
func (h *ChallengeFileHandler) renderFileList(w http.ResponseWriter, challengeID string) {
	files, err := h.db.GetChallengeFiles(challengeID)
	if err != nil {
		http.Error(w, "Failed to fetch files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	if len(files) == 0 {
		w.Write([]byte(`<div class="text-gray-400 text-sm">No files attached.</div>`))
		return
	}

	html := `<div class="space-y-2">`
	for _, f := range files {
		sizeStr := ""
		if f.SizeBytes != nil && *f.SizeBytes > 0 {
			if *f.SizeBytes < 1024 {
				sizeStr = fmt.Sprintf(" (%d bytes)", *f.SizeBytes)
			} else if *f.SizeBytes < 1024*1024 {
				sizeStr = fmt.Sprintf(" (%.1f KB)", float64(*f.SizeBytes)/1024)
			} else {
				sizeStr = fmt.Sprintf(" (%.1f MB)", float64(*f.SizeBytes)/(1024*1024))
			}
		}
		html += fmt.Sprintf(`<div class="flex items-center justify-between bg-dark-bg border border-dark-border rounded p-2">
			<div class="flex items-center gap-2">
				<span class="text-green-400 text-sm">📎 %s%s</span>
				<a href="%s" class="text-blue-400 hover:text-blue-300 text-sm underline" target="_blank">Download</a>
			</div>
			<button hx-delete="/api/admin/challenge-files/%s" hx-target="#challenge-files-list" hx-swap="outerHTML" class="text-red-400 hover:text-red-300 text-xs">Remove</button>
		</div>`, f.Filename, sizeStr, f.StoragePath, f.ID)
	}
	html += `</div>`
	w.Write([]byte(html))
}

// BatchUpload handles POST /api/admin/challenges/{id}/files/batch
// Uploads multiple files at once from the edit form
func (h *ChallengeFileHandler) BatchUpload(w http.ResponseWriter, r *http.Request) {
	challengeID := chi.URLParam(r, "id")

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB total
		http.Error(w, "Files too large (max 100MB total)", http.StatusBadRequest)
		return
	}

	// Process multiple files
	for i := 0; ; i++ {
		sourceKey := fmt.Sprintf("newfile_%d_source", i)
		source := r.FormValue(sourceKey)
		if source == "" {
			break // No more files
		}

		if source == "upload" {
			fileKey := fmt.Sprintf("newfile_%d_file", i)
			if file, header, err := r.FormFile(fileKey); err == nil {
				url, uploadErr := h.storage.Upload(r.Context(), header.Filename, file)
				file.Close()
				if uploadErr == nil {
					sizeBytes := header.Size
					if _, err := h.db.CreateChallengeFile(challengeID, header.Filename, "local", url, &sizeBytes); err != nil {
						log.Printf("warning: failed to save file record for %s: %v", header.Filename, err)
					}
				}
			}
		} else if source == "external" {
			urlKey := fmt.Sprintf("newfile_%d_url", i)
			nameKey := fmt.Sprintf("newfile_%d_name", i)
			externalURL := r.FormValue(urlKey)
			if externalURL != "" {
				filename := r.FormValue(nameKey)
				if filename == "" {
					filename = "external-file"
				}
				if _, err := h.db.CreateChallengeFile(challengeID, filename, "external", externalURL, nil); err != nil {
					log.Printf("warning: failed to save external file record for %s: %v", filename, err)
				}
			}
		}
	}

	// Return updated file list
	h.renderFileList(w, challengeID)
}
