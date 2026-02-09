package api

import (
	"encoding/json"
	"net/http"

	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
)

type UserHandler struct {
	DB *db.DB
}

type UserResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		// Should be handled by middleware usually, but unexpected assertion fail
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.DB.GetUserByID(userID)
	if err != nil {
		JSONError(w, "User not found", http.StatusNotFound)
		return
	}

	resp := UserResponse{
		ID:    user.ID,
		Email: user.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
