package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ajesus37/hCTF2/internal/database"
)

type SQLHandler struct {
	db *database.DB
}

func NewSQLHandler(db *database.DB) *SQLHandler {
	return &SQLHandler{db: db}
}

// GetSnapshot godoc
// @Summary Get a snapshot of the SQL playground database
// @Description Returns the current state of the SQL playground database for use with DuckDB WASM.
// @Tags SQL
// @Produce json
// @Security CookieAuth
// @Success 200 {object} object
// @Failure 500 {object} object{error=string}
// @Router /sql/snapshot [get]
func (h *SQLHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.db.GetSQLSnapshot()
	if err != nil {
		http.Error(w, "Failed to generate snapshot", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}
