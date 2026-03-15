package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ajesus37/hCTF2/internal/auth"
	"github.com/ajesus37/hCTF2/internal/database"
	"github.com/ajesus37/hCTF2/internal/models"
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

// ListCategories godoc
// @Summary List all challenge categories
// @Tags Settings
// @Produce json
// @Success 200 {array} models.CategoryOption
// @Router /categories [get]
func (h *SettingsHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.db.GetAllCategories()
	if err != nil {
		http.Error(w, "failed to fetch categories", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cats)
}

// ListDifficulties godoc
// @Summary List all challenge difficulties
// @Tags Settings
// @Produce json
// @Success 200 {array} models.DifficultyOption
// @Router /difficulties [get]
func (h *SettingsHandler) ListDifficulties(w http.ResponseWriter, r *http.Request) {
	diffs, err := h.db.GetAllDifficulties()
	if err != nil {
		http.Error(w, "failed to fetch difficulties", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diffs)
}

// CreateCategory godoc
// @Summary Create a new challenge category (admin only)
// @Tags Admin
// @Accept application/x-www-form-urlencoded
// @Produce html
// @Security CookieAuth
// @Param name formData string true "Category name"
// @Param sort_order formData integer false "Sort order (default 0)"
// @Success 200 {string} string "HTML fragment of the new category row"
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /admin/categories [post]
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
		if v, err := strconv.Atoi(s); err == nil { sortOrder = v }
	}

	cat, err := h.db.CreateCategory(name, sortOrder)
	if err != nil {
		http.Error(w, "Failed to create category (may already exist)", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderCategoryHTML(cat)))
}

// UpdateCategory godoc
// @Summary Update a challenge category (admin only)
// @Tags Admin
// @Accept application/x-www-form-urlencoded
// @Produce html
// @Security CookieAuth
// @Param id path string true "Category ID"
// @Param name formData string true "Category name"
// @Param sort_order formData integer false "Sort order"
// @Success 200 {string} string "HTML fragment of the updated category row"
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /admin/categories/{id} [put]
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
		if v, err := strconv.Atoi(s); err == nil { sortOrder = v }
	}

	cat, err := h.db.UpdateCategory(id, name, sortOrder)
	if err != nil {
		http.Error(w, "Failed to update category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderCategoryHTML(cat)))
}

// DeleteCategory godoc
// @Summary Delete a challenge category (admin only)
// @Tags Admin
// @Produce plain
// @Security CookieAuth
// @Param id path string true "Category ID"
// @Success 200 {string} string "Empty response on success"
// @Failure 500 {object} object{error=string}
// @Router /admin/categories/{id} [delete]
func (h *SettingsHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteCategory(id); err != nil {
		http.Error(w, "Failed to delete category", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// CreateDifficulty godoc
// @Summary Create a new difficulty level (admin only)
// @Tags Admin
// @Accept application/x-www-form-urlencoded
// @Produce html
// @Security CookieAuth
// @Param name formData string true "Difficulty name"
// @Param color formData string false "Badge color Tailwind classes"
// @Param text_color formData string false "Text color Tailwind class"
// @Param sort_order formData integer false "Sort order (default 0)"
// @Success 200 {string} string "HTML fragment of the new difficulty row"
// @Failure 400 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /admin/difficulties [post]
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
		if v, err := strconv.Atoi(s); err == nil { sortOrder = v }
	}

	diff, err := h.db.CreateDifficulty(name, color, textColor, sortOrder)
	if err != nil {
		http.Error(w, "Failed to create difficulty (may already exist)", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderDifficultyHTML(diff)))
}

// UpdateDifficulty godoc
// @Summary Update a difficulty level (admin only)
// @Tags Admin
// @Accept application/x-www-form-urlencoded
// @Produce html
// @Security CookieAuth
// @Param id path string true "Difficulty ID"
// @Param name formData string true "Difficulty name"
// @Param color formData string false "Badge color Tailwind classes"
// @Param text_color formData string false "Text color Tailwind class"
// @Param sort_order formData integer false "Sort order"
// @Success 200 {string} string "HTML fragment of the updated difficulty row"
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /admin/difficulties/{id} [put]
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
		if v, err := strconv.Atoi(s); err == nil { sortOrder = v }
	}

	diff, err := h.db.UpdateDifficulty(id, name, color, textColor, sortOrder)
	if err != nil {
		http.Error(w, "Failed to update difficulty", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderDifficultyHTML(diff)))
}

// DeleteDifficulty godoc
// @Summary Delete a difficulty level (admin only)
// @Tags Admin
// @Produce plain
// @Security CookieAuth
// @Param id path string true "Difficulty ID"
// @Success 200 {string} string "Empty response on success"
// @Failure 500 {object} object{error=string}
// @Router /admin/difficulties/{id} [delete]
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

// GetCustomCode godoc
// @Summary Get current custom HTML/JS injection settings (admin only)
// @Tags Admin
// @Produce json
// @Security CookieAuth
// @Success 200 {object} object{head_html=string,body_end_html=string,pages_json=string,motd=string}
// @Failure 500 {object} object{error=string}
// @Router /admin/custom-code [get]
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

// UpdateCustomCode godoc
// @Summary Update custom HTML/JS injection settings (admin only)
// @Tags Admin
// @Accept application/x-www-form-urlencoded
// @Produce plain
// @Security CookieAuth
// @Param head_html formData string false "HTML to inject in <head>"
// @Param body_end_html formData string false "HTML to inject before </body>"
// @Param pages_json formData string false "JSON config for per-page injection"
// @Param motd formData string false "Message of the day"
// @Success 200 {string} string "Settings updated successfully"
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /admin/custom-code [put]
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

// renderUserHTML returns the HTML fragment for a user row in the admin user management panel.
func renderUserHTML(w http.ResponseWriter, u models.User) {
	adminBadge := `<span class="px-2 py-1 text-xs rounded bg-purple-600 text-white">Admin</span>`
	userBadge := `<span class="px-2 py-1 text-xs rounded bg-gray-600 text-gray-300">User</span>`
	badge := userBadge
	btnClass := "bg-purple-600 hover:bg-purple-700"
	btnText := "Promote"
	if u.IsAdmin {
		badge = adminBadge
		btnClass = "bg-yellow-600 hover:bg-yellow-700"
		btnText = "Demote"
	}

	fmt.Fprintf(w, `<div id="user-%s" class="bg-dark-surface border border-dark-border rounded-lg p-4">
		<div class="flex justify-between items-center">
			<div>
				<h4 class="font-bold text-white">%s</h4>
				<p class="text-sm text-gray-400">%s</p>
				<p class="text-xs text-gray-500 mt-1">Joined: %s</p>
			</div>
			<div class="flex items-center gap-3">
				%s
				<button hx-put="/api/admin/users/%s/admin" hx-target="#user-%s" hx-swap="outerHTML"
					class="px-3 py-1 %s text-white rounded text-sm font-medium transition">
					%s
				</button>
				<button hx-delete="/api/admin/users/%s" hx-target="#user-%s" hx-swap="outerHTML swap:0.5s"
					hx-confirm="Delete user '%s'? This action cannot be undone."
					class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">
					Delete
				</button>
			</div>
		</div>
	</div>`,
		u.ID, html.EscapeString(u.Name), html.EscapeString(u.Email),
		u.CreatedAt.Format("2006-01-02"), badge, u.ID, u.ID,
		btnClass, btnText, u.ID, u.ID, html.EscapeString(u.Name))
}

// ListUsers godoc
// @Summary List all users (admin only)
// @Tags Admin
// @Produce html
// @Security CookieAuth
// @Success 200 {string} string "HTML fragments with user rows"
// @Failure 500 {object} object{error=string}
// @Router /admin/users [get]
func (h *SettingsHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	// Content negotiation: return JSON for API clients, HTML for HTMX
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	for _, u := range users {
		renderUserHTML(w, u)
	}

	if len(users) == 0 {
		w.Write([]byte(`<div class="text-center py-8 text-gray-400"><p>No users found.</p></div>`))
	}
}

// UpdateUserAdmin godoc
// @Summary Toggle admin status for a user (admin only)
// @Tags Admin
// @Produce html
// @Security CookieAuth
// @Param id path string true "User ID"
// @Success 200 {string} string "HTML fragment of the updated user row"
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /admin/users/{id}/admin [put]
func (h *SettingsHandler) UpdateUserAdmin(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	// Prevent self-modification
	if claims != nil && claims.UserID == userID {
		http.Error(w, "Cannot modify your own admin status", http.StatusForbidden)
		return
	}

	// Get current user to toggle status
	user, err := h.db.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Toggle admin status
	newStatus := !user.IsAdmin
	if err := h.db.SetUserAdminStatus(userID, newStatus); err != nil {
		http.Error(w, "Failed to update admin status", http.StatusInternalServerError)
		return
	}

	// Return updated user HTML
	user.IsAdmin = newStatus
	w.Header().Set("Content-Type", "text/html")
	renderUserHTML(w, *user)
}

// DeleteUser godoc
// @Summary Delete a user account (admin only)
// @Tags Admin
// @Produce plain
// @Security CookieAuth
// @Param id path string true "User ID"
// @Success 200 {string} string "Empty response on success"
// @Failure 403 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /admin/users/{id} [delete]
func (h *SettingsHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	// Prevent self-deletion
	if claims != nil && claims.UserID == userID {
		http.Error(w, "Cannot delete yourself", http.StatusForbidden)
		return
	}

	if err := h.db.DeleteUser(userID); err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// SetScoreFreeze handles POST /api/admin/settings/freeze
func (h *SettingsHandler) SetScoreFreeze(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	enabled := r.FormValue("freeze_enabled") == "1"
	freezeAtStr := r.FormValue("freeze_at")

	var freezeAt *time.Time
	if freezeAtStr != "" {
		t, err := time.Parse("2006-01-02T15:04", freezeAtStr)
		if err == nil {
			ft := t.UTC()
			freezeAt = &ft
		}
	}

	if err := h.db.SetScoreFreeze(enabled, freezeAt); err != nil {
		http.Error(w, "Failed to save", http.StatusInternalServerError)
		return
	}

	frozen := h.db.IsFrozen()
	statusText := "Live"
	statusClass := "text-green-400"
	if frozen {
		statusText = "Frozen"
		statusClass = "text-blue-400"
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<span id="freeze-status" class="%s font-semibold">%s</span>`, statusClass, statusText)
}

// SetAdminVisibility handles POST /api/admin/settings/admin-visibility
func (h *SettingsHandler) SetAdminVisibility(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Checkbox sends "1" when checked, nothing when unchecked
	visible := r.FormValue("admin_visible") == "1"

	if err := h.db.SetAdminVisibleInScoreboard(visible); err != nil {
		http.Error(w, "Failed to save", http.StatusInternalServerError)
		return
	}

	statusText := "Hidden"
	if visible {
		statusText = "Visible"
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<span id="admin-visibility-status" class="font-semibold">%s</span>`, statusText)
}
