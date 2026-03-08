package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ajesus37/hCTF2/internal/database"
	"github.com/ajesus37/hCTF2/internal/models"
)

type ImportExportHandler struct {
	db *database.DB
}

func NewImportExportHandler(db *database.DB) *ImportExportHandler {
	return &ImportExportHandler{db: db}
}

// ExportChallenges handles GET /api/admin/export
func (h *ImportExportHandler) ExportChallenges(w http.ResponseWriter, r *http.Request) {
	bundle, err := h.db.ExportBundle()
	if err != nil {
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hctf2-export-%s.json"`, time.Now().Format("2006-01-02")))
	json.NewEncoder(w).Encode(bundle)
}

// ImportChallenges handles POST /api/admin/import
func (h *ImportExportHandler) ImportChallenges(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	var bundle models.ExportBundle
	if err := json.NewDecoder(file).Decode(&bundle); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if bundle.Version != 1 {
		http.Error(w, "Unsupported export version", http.StatusBadRequest)
		return
	}

	result, err := h.db.ImportBundle(bundle.Categories, bundle.Difficulties, bundle.Challenges)
	if err != nil {
		http.Error(w, "Import failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return HTMX-friendly result
	w.Header().Set("Content-Type", "text/html")
	html := fmt.Sprintf(`<div class="p-4 bg-green-900/50 border border-green-700 rounded text-sm text-green-300">
		Imported %d challenge(s).`, result.Imported)
	if len(result.Renamed) > 0 {
		html += `</div><div class="mt-2 p-4 bg-yellow-900/50 border border-yellow-700 rounded text-sm text-yellow-300"><strong>Renamed (duplicates):</strong><ul class="list-disc ml-4 mt-1">`
		for _, r := range result.Renamed {
			html += fmt.Sprintf("<li>%s</li>", r)
		}
		html += `</ul>`
	}
	if len(result.Errors) > 0 {
		html += `</div><div class="mt-2 p-4 bg-red-900/50 border border-red-700 rounded text-sm text-red-300"><strong>Errors:</strong><ul class="list-disc ml-4 mt-1">`
		for _, e := range result.Errors {
			html += fmt.Sprintf("<li>%s</li>", e)
		}
		html += `</ul>`
	}
	html += `</div>`
	w.Write([]byte(html))
}
