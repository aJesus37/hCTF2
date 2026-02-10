package handlers

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yourusername/hctf2/internal/database"
	"github.com/yourusername/hctf2/internal/models"
)

type SettingsHandler struct {
	db *database.DB
}

func NewSettingsHandler(db *database.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

// renderCategoryHTML returns the HTML fragment for a category row with inline edit support.
func renderCategoryHTML(cat *models.CategoryOption) string {
	eName := html.EscapeString(cat.Name)
	return fmt.Sprintf(`<div id="cat-%s" x-data="{ editing: false }" class="bg-dark-bg border border-dark-border rounded px-4 py-2">
	<div x-show="!editing" class="flex items-center justify-between">
		<span class="text-white">%s <span class="text-xs text-gray-400">(order: %d)</span></span>
		<div class="flex gap-2">
			<button @click="editing = true" class="px-2 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-xs">Edit</button>
			<button hx-delete="/api/admin/categories/%s" hx-target="#cat-%s" hx-swap="outerHTML" hx-confirm="Delete category '%s'?"
				class="px-2 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-xs">Delete</button>
		</div>
	</div>
	<form x-show="editing" hx-put="/api/admin/categories/%s" hx-target="#cat-%s" hx-swap="outerHTML"
		class="flex gap-3 items-end">
		<div>
			<label class="block text-xs text-gray-300 mb-1">Name</label>
			<input type="text" name="name" value="%s" required
				class="px-3 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm">
		</div>
		<div>
			<label class="block text-xs text-gray-300 mb-1">Sort Order</label>
			<input type="number" name="sort_order" value="%d" min="0"
				class="w-20 px-3 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm">
		</div>
		<button type="submit" class="px-3 py-2 bg-green-600 hover:bg-green-700 text-white rounded text-xs font-medium">Save</button>
		<button type="button" @click="editing = false" class="px-3 py-2 bg-gray-600 hover:bg-gray-700 text-white rounded text-xs font-medium">Cancel</button>
	</form>
</div>`, cat.ID, eName, cat.SortOrder, cat.ID, cat.ID, eName, cat.ID, cat.ID, eName, cat.SortOrder)
}

// renderDifficultyHTML returns the HTML fragment for a difficulty row with inline edit support.
func renderDifficultyHTML(diff *models.DifficultyOption) string {
	eName := html.EscapeString(diff.Name)
	eColor := html.EscapeString(diff.Color)
	eTextColor := html.EscapeString(diff.TextColor)
	return fmt.Sprintf(`<div id="diff-%s" x-data="{ editing: false }" class="bg-dark-bg border border-dark-border rounded px-4 py-2">
	<div x-show="!editing" class="flex items-center justify-between">
		<div>
			<span class="%s font-medium">%s</span>
			<span class="text-xs text-gray-400 ml-2">(order: %d)</span>
			<span class="ml-2 px-2 py-0.5 text-xs rounded %s">preview</span>
		</div>
		<div class="flex gap-2">
			<button @click="editing = true" class="px-2 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-xs">Edit</button>
			<button hx-delete="/api/admin/difficulties/%s" hx-target="#diff-%s" hx-swap="outerHTML" hx-confirm="Delete difficulty '%s'?"
				class="px-2 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-xs">Delete</button>
		</div>
	</div>
	<form x-show="editing" hx-put="/api/admin/difficulties/%s" hx-target="#diff-%s" hx-swap="outerHTML"
		class="space-y-3">
		<div class="grid grid-cols-2 gap-3">
			<div>
				<label class="block text-xs text-gray-300 mb-1">Name</label>
				<input type="text" name="name" value="%s" required
					class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm">
			</div>
			<div>
				<label class="block text-xs text-gray-300 mb-1">Sort Order</label>
				<input type="number" name="sort_order" value="%d" min="0"
					class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm">
			</div>
		</div>
		<div class="grid grid-cols-2 gap-3">
			<div>
				<label class="block text-xs text-gray-300 mb-1">Badge Color (Tailwind classes)</label>
				<input type="text" name="color" value="%s"
					class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm">
			</div>
			<div>
				<label class="block text-xs text-gray-300 mb-1">Text Color (Tailwind class)</label>
				<input type="text" name="text_color" value="%s"
					class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm">
			</div>
		</div>
		<div class="flex gap-2">
			<button type="submit" class="px-3 py-2 bg-green-600 hover:bg-green-700 text-white rounded text-xs font-medium">Save</button>
			<button type="button" @click="editing = false" class="px-3 py-2 bg-gray-600 hover:bg-gray-700 text-white rounded text-xs font-medium">Cancel</button>
		</div>
	</form>
</div>`, diff.ID, eTextColor, eName, diff.SortOrder, eColor, diff.ID, diff.ID, eName, diff.ID, diff.ID, eName, diff.SortOrder, eColor, eTextColor)
}

func (h *SettingsHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	sortOrder := 0
	if s := r.FormValue("sort_order"); s != "" {
		fmt.Sscanf(s, "%d", &sortOrder)
	}

	cat, err := h.db.CreateCategory(name, sortOrder)
	if err != nil {
		http.Error(w, "Failed to create category (may already exist)", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderCategoryHTML(cat)))
}

func (h *SettingsHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	sortOrder := 0
	if s := r.FormValue("sort_order"); s != "" {
		fmt.Sscanf(s, "%d", &sortOrder)
	}

	cat, err := h.db.UpdateCategory(id, name, sortOrder)
	if err != nil {
		http.Error(w, "Failed to update category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderCategoryHTML(cat)))
}

func (h *SettingsHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteCategory(id); err != nil {
		http.Error(w, "Failed to delete category", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

func (h *SettingsHandler) CreateDifficulty(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	color := strings.TrimSpace(r.FormValue("color"))
	if color == "" {
		color = "bg-gray-600 text-gray-100"
	}

	textColor := strings.TrimSpace(r.FormValue("text_color"))
	if textColor == "" {
		textColor = "text-gray-400"
	}

	sortOrder := 0
	if s := r.FormValue("sort_order"); s != "" {
		fmt.Sscanf(s, "%d", &sortOrder)
	}

	diff, err := h.db.CreateDifficulty(name, color, textColor, sortOrder)
	if err != nil {
		http.Error(w, "Failed to create difficulty (may already exist)", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderDifficultyHTML(diff)))
}

func (h *SettingsHandler) UpdateDifficulty(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	color := strings.TrimSpace(r.FormValue("color"))
	if color == "" {
		color = "bg-gray-600 text-gray-100"
	}

	textColor := strings.TrimSpace(r.FormValue("text_color"))
	if textColor == "" {
		textColor = "text-gray-400"
	}

	sortOrder := 0
	if s := r.FormValue("sort_order"); s != "" {
		fmt.Sscanf(s, "%d", &sortOrder)
	}

	diff, err := h.db.UpdateDifficulty(id, name, color, textColor, sortOrder)
	if err != nil {
		http.Error(w, "Failed to update difficulty", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderDifficultyHTML(diff)))
}

func (h *SettingsHandler) DeleteDifficulty(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteDifficulty(id); err != nil {
		http.Error(w, "Failed to delete difficulty", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// Custom code injection handlers

func (h *SettingsHandler) GetCustomCode(w http.ResponseWriter, r *http.Request) {
	headHTML, _ := h.db.GetSetting("custom_head_html")
	bodyEndHTML, _ := h.db.GetSetting("custom_body_end_html")
	pagesJSON, _ := h.db.GetSetting("custom_code_pages")
	motd, _ := h.db.GetSetting("motd")

	data := map[string]string{
		"head_html":     headHTML,
		"body_end_html": bodyEndHTML,
		"pages_json":    pagesJSON,
		"motd":          motd,
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"head_html":%q,"body_end_html":%q,"pages_json":%q,"motd":%q}`,
		data["head_html"], data["body_end_html"], data["pages_json"], data["motd"])
}

func (h *SettingsHandler) UpdateCustomCode(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	headHTML := r.FormValue("head_html")
	bodyEndHTML := r.FormValue("body_end_html")
	pagesJSON := r.FormValue("pages_json")
	motd := r.FormValue("motd")

	if r.Form.Has("head_html") {
		if err := h.db.SetSetting("custom_head_html", headHTML); err != nil {
			http.Error(w, "Failed to save head HTML", http.StatusInternalServerError)
			return
		}
	}

	if r.Form.Has("body_end_html") {
		if err := h.db.SetSetting("custom_body_end_html", bodyEndHTML); err != nil {
			http.Error(w, "Failed to save body HTML", http.StatusInternalServerError)
			return
		}
	}

	if r.Form.Has("pages_json") {
		if err := h.db.SetSetting("custom_code_pages", pagesJSON); err != nil {
			http.Error(w, "Failed to save pages config", http.StatusInternalServerError)
			return
		}
	}

	if r.Form.Has("motd") {
		if err := h.db.SetSetting("motd", motd); err != nil {
			http.Error(w, "Failed to save MOTD", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Settings updated successfully"))
}
