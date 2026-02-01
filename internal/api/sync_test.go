package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/theLastOfCats/kotatsu-go-server/internal/model"
	"github.com/theLastOfCats/kotatsu-go-server/internal/testutil"
)

func TestSyncHistory(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	// Setup User
	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "hist@example.com", "hash")
	userID, _ := res.LastInsertId()

	handler := &SyncHandler{DB: database}

	// 1. Post History
	now := time.Now().UnixMilli()
	manga := model.Manga{
		ID: 100, Title: "Test Manga", URL: "http://manga", PublicURL: "http://public", Rating: 5, Source: "test", CoverURL: "http://cover",
	}
	historyItem := model.History{
		MangaID: 100, Manga: &manga, CreatedAt: now, UpdatedAt: now, ChapterID: 1, Page: 1, Chapters: 10, DeletedAt: 0,
	}
	payload := model.HistoryPackage{
		History: []model.History{historyItem},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/resource/history", bytes.NewBuffer(body))
	// Mock Auth Middleware
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))

	rr := httptest.NewRecorder()
	handler.PostHistory(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("PostHistory failed code: %v body: %s", status, rr.Body.String())
	}

	var resp model.HistoryPackage
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.History) != 1 {
		t.Error("Expected 1 history item returned")
	}
	if resp.History[0].Manga.Title != "Test Manga" {
		t.Error("Manga detail mismatch")
	}
	if resp.Timestamp == nil {
		t.Error("Expected sync timestamp")
	}

	// 2. Get History
	reqGet, _ := http.NewRequest("GET", "/resource/history", nil)
	reqGet = reqGet.WithContext(context.WithValue(reqGet.Context(), UserIDKey, userID))
	rrGet := httptest.NewRecorder()

	handler.GetHistory(rrGet, reqGet)
	if status := rrGet.Code; status != http.StatusOK {
		t.Errorf("GetHistory failed: %v", status)
	}

	var respGet model.HistoryPackage
	json.NewDecoder(rrGet.Body).Decode(&respGet)
	if len(respGet.History) != 1 {
		t.Errorf("Expected 1 history item, got %d", len(respGet.History))
	}
}

func TestSyncFavourites(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	// Setup User
	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "fav@example.com", "hash")
	userID, _ := res.LastInsertId()

	handler := &SyncHandler{DB: database}

	// 1. Post Favourites & Categories
	now := time.Now().UnixMilli()
	cat := model.Category{
		ID: 1, SortKey: 0, Title: "Reading", Order: "asc", Track: true, ShowInLib: true, CreatedAt: now,
	}
	manga := model.Manga{
		ID: 200, Title: "Fav Manga", URL: "http://fav", PublicURL: "http://fav", Rating: 4.5, Source: "test", CoverURL: "http://cover",
	}
	fav := model.Favourite{
		MangaID: 200, Manga: &manga, CategoryID: 1, SortKey: 0, Pinned: false, CreatedAt: now, DeletedAt: 0,
	}

	payload := model.FavouritesPackage{
		Categories: []model.Category{cat},
		Favourites: []model.Favourite{fav},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))
	rr := httptest.NewRecorder()

	handler.PostFavourites(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("PostFavourites failed: %v body: %s", status, rr.Body.String())
	}

	// 2. Get Favourites
	reqGet, _ := http.NewRequest("GET", "/resource/favourites", nil)
	reqGet = reqGet.WithContext(context.WithValue(reqGet.Context(), UserIDKey, userID))
	rrGet := httptest.NewRecorder()

	handler.GetFavourites(rrGet, reqGet)
	if status := rrGet.Code; status != http.StatusOK {
		t.Errorf("GetFavourites failed: %v", status)
	}

	var respGet model.FavouritesPackage
	json.NewDecoder(rrGet.Body).Decode(&respGet)

	if len(respGet.Categories) != 1 || respGet.Categories[0].Title != "Reading" {
		t.Error("Category sync mismatch")
	}
	if len(respGet.Favourites) != 1 || respGet.Favourites[0].Manga.Title != "Fav Manga" {
		t.Error("Favourite sync mismatch")
	}
}
