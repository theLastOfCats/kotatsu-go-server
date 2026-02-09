package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSyncFavouritesOlderCategoryDoesNotOverwrite(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "multi-device-cat@example.com", "hash")
	userID, _ := res.LastInsertId()
	handler := &SyncHandler{DB: database}

	deletedAt := int64(0)
	catPhone1 := model.Category{
		ID:        1,
		CreatedAt: 1770657807527,
		SortKey:   1,
		Title:     "Read later",
		Order:     "NEWEST",
		Track:     true,
		ShowInLib: true,
		DeletedAt: &deletedAt,
	}
	payloadPhone1 := model.FavouritesPackage{Categories: []model.Category{catPhone1}}

	bodyPhone1, _ := json.Marshal(payloadPhone1)
	reqPhone1, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(bodyPhone1))
	reqPhone1 = reqPhone1.WithContext(context.WithValue(reqPhone1.Context(), UserIDKey, userID))
	rrPhone1 := httptest.NewRecorder()
	handler.PostFavourites(rrPhone1, reqPhone1)
	if rrPhone1.Code != http.StatusOK {
		t.Fatalf("phone1 PostFavourites failed: %d body: %s", rrPhone1.Code, rrPhone1.Body.String())
	}

	catPhone2Older := catPhone1
	catPhone2Older.CreatedAt = 1769951242943
	payloadPhone2 := model.FavouritesPackage{Categories: []model.Category{catPhone2Older}}

	bodyPhone2, _ := json.Marshal(payloadPhone2)
	reqPhone2, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(bodyPhone2))
	reqPhone2 = reqPhone2.WithContext(context.WithValue(reqPhone2.Context(), UserIDKey, userID))
	rrPhone2 := httptest.NewRecorder()
	handler.PostFavourites(rrPhone2, reqPhone2)
	if rrPhone2.Code != http.StatusOK {
		t.Fatalf("phone2 PostFavourites failed: %d body: %s", rrPhone2.Code, rrPhone2.Body.String())
	}

	var createdAt int64
	if err := database.QueryRow("SELECT created_at FROM categories WHERE id = ? AND user_id = ?", 1, userID).Scan(&createdAt); err != nil {
		t.Fatalf("failed to read category: %v", err)
	}
	if createdAt != catPhone1.CreatedAt {
		t.Fatalf("category was overwritten by older payload: got %d want %d", createdAt, catPhone1.CreatedAt)
	}
}

func TestSyncFavouritesTombstoneNotOverwrittenByOlderState(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "multi-device-fav@example.com", "hash")
	userID, _ := res.LastInsertId()
	handler := &SyncHandler{DB: database}

	deletedAt := int64(0)
	category := model.Category{
		ID:        1,
		CreatedAt: 1770657807527,
		SortKey:   1,
		Title:     "Read later",
		Order:     "NEWEST",
		Track:     true,
		ShowInLib: true,
		DeletedAt: &deletedAt,
	}
	manga := model.Manga{
		ID:        -4321889287621624117,
		Title:     "Records of the Swordsman Scholar",
		URL:       "/manga/records-of-the-swordsman-scholar/",
		PublicURL: "https://mangagojo.com/manga/records-of-the-swordsman-scholar/",
		Rating:    0.7,
		Source:    "MANGAGOJO",
		CoverURL:  "https://mangagojo.com/wp-content/uploads/2024/05/records-of-the-swordsman-scholar.jpg",
	}

	favDeleted := model.Favourite{
		MangaID:    manga.ID,
		Manga:      &manga,
		CategoryID: 1,
		SortKey:    0,
		Pinned:     false,
		CreatedAt:  1770655770283,
		DeletedAt:  1770658380470,
	}
	payloadPhone1 := model.FavouritesPackage{
		Categories: []model.Category{category},
		Favourites: []model.Favourite{favDeleted},
	}

	bodyPhone1, _ := json.Marshal(payloadPhone1)
	reqPhone1, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(bodyPhone1))
	reqPhone1 = reqPhone1.WithContext(context.WithValue(reqPhone1.Context(), UserIDKey, userID))
	rrPhone1 := httptest.NewRecorder()
	handler.PostFavourites(rrPhone1, reqPhone1)
	if rrPhone1.Code != http.StatusOK {
		t.Fatalf("phone1 PostFavourites failed: %d body: %s", rrPhone1.Code, rrPhone1.Body.String())
	}

	favOlderActive := favDeleted
	favOlderActive.DeletedAt = 0
	favOlderActive.Manga = &manga
	payloadPhone2 := model.FavouritesPackage{
		Categories: []model.Category{category},
		Favourites: []model.Favourite{favOlderActive},
	}

	bodyPhone2, _ := json.Marshal(payloadPhone2)
	reqPhone2, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(bodyPhone2))
	reqPhone2 = reqPhone2.WithContext(context.WithValue(reqPhone2.Context(), UserIDKey, userID))
	rrPhone2 := httptest.NewRecorder()
	handler.PostFavourites(rrPhone2, reqPhone2)
	if rrPhone2.Code != http.StatusOK {
		t.Fatalf("phone2 PostFavourites failed: %d body: %s", rrPhone2.Code, rrPhone2.Body.String())
	}

	var dbDeletedAt int64
	if err := database.QueryRow("SELECT deleted_at FROM favourites WHERE manga_id = ? AND category_id = ? AND user_id = ?", manga.ID, category.ID, userID).Scan(&dbDeletedAt); err != nil {
		t.Fatalf("failed to read favourite: %v", err)
	}
	if dbDeletedAt != favDeleted.DeletedAt {
		t.Fatalf("favourite tombstone was overwritten: got %d want %d", dbDeletedAt, favDeleted.DeletedAt)
	}
}

func TestSyncHandlersUnauthorized(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	handler := &SyncHandler{DB: database}

	tests := []struct {
		name   string
		method string
		path   string
		call   func(http.ResponseWriter, *http.Request)
		body   string
	}{
		{name: "GetHistory", method: "GET", path: "/resource/history", call: handler.GetHistory},
		{name: "PostHistory", method: "POST", path: "/resource/history", call: handler.PostHistory, body: `{"history":[]}`},
		{name: "GetFavourites", method: "GET", path: "/resource/favourites", call: handler.GetFavourites},
		{name: "PostFavourites", method: "POST", path: "/resource/favourites", call: handler.PostFavourites, body: `{"categories":[],"favourites":[]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			rr := httptest.NewRecorder()

			tc.call(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestPostSyncHandlersRejectInvalidJSON(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "badjson@example.com", "hash")
	userID, _ := res.LastInsertId()
	handler := &SyncHandler{DB: database}

	tests := []struct {
		name   string
		path   string
		call   func(http.ResponseWriter, *http.Request)
	}{
		{name: "PostHistory", path: "/resource/history", call: handler.PostHistory},
		{name: "PostFavourites", path: "/resource/favourites", call: handler.PostFavourites},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", tc.path, strings.NewReader("{bad json"))
			req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))
			rr := httptest.NewRecorder()

			tc.call(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestPostHistoryOlderUpdateDoesNotOverwriteNewer(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "history-order@example.com", "hash")
	userID, _ := res.LastInsertId()
	handler := &SyncHandler{DB: database}

	manga := model.Manga{
		ID: 777, Title: "Ordering", URL: "http://ordering", PublicURL: "http://ordering", Rating: 5, Source: "test", CoverURL: "http://cover",
	}

	newer := model.History{
		MangaID: 777, Manga: &manga, CreatedAt: 100, UpdatedAt: 200, ChapterID: 5, Page: 12, Chapters: 30, DeletedAt: 0,
	}
	older := model.History{
		MangaID: 777, Manga: &manga, CreatedAt: 100, UpdatedAt: 100, ChapterID: 1, Page: 2, Chapters: 30, DeletedAt: 0,
	}

	postHistoryPackage(t, handler, userID, model.HistoryPackage{History: []model.History{newer}})
	postHistoryPackage(t, handler, userID, model.HistoryPackage{History: []model.History{older}})

	var updatedAt int64
	var page int
	if err := database.QueryRow("SELECT updated_at, page FROM history WHERE user_id = ? AND manga_id = ?", userID, manga.ID).Scan(&updatedAt, &page); err != nil {
		t.Fatalf("failed to read history row: %v", err)
	}
	if updatedAt != newer.UpdatedAt || page != newer.Page {
		t.Fatalf("older payload overwrote newer row: got updated_at=%d page=%d", updatedAt, page)
	}
}

func TestPostFavouritesEnsuresMissingCategoryAndManga(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "placeholder@example.com", "hash")
	userID, _ := res.LastInsertId()
	handler := &SyncHandler{DB: database}

	payload := model.FavouritesPackage{
		Favourites: []model.Favourite{
			{
				MangaID:    99999,
				CategoryID: 123,
				SortKey:    0,
				Pinned:     false,
				CreatedAt:  1000,
				DeletedAt:  0,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))
	rr := httptest.NewRecorder()
	handler.PostFavourites(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PostFavourites failed: %d body: %s", rr.Code, rr.Body.String())
	}

	var categoryTitle string
	if err := database.QueryRow("SELECT title FROM categories WHERE id = ? AND user_id = ?", 123, userID).Scan(&categoryTitle); err != nil {
		t.Fatalf("missing ensured category: %v", err)
	}
	if categoryTitle != "Unknown" {
		t.Fatalf("expected ensured category title to be Unknown, got %q", categoryTitle)
	}

	var mangaTitle string
	if err := database.QueryRow("SELECT title FROM manga WHERE id = ?", 99999).Scan(&mangaTitle); err != nil {
		t.Fatalf("missing ensured manga: %v", err)
	}
}

func TestGetFavouritesIsUserScoped(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	res1, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "u1@example.com", "hash")
	userID1, _ := res1.LastInsertId()
	res2, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "u2@example.com", "hash")
	userID2, _ := res2.LastInsertId()

	handler := &SyncHandler{DB: database}
	now := time.Now().UnixMilli()

	payload1 := model.FavouritesPackage{
		Categories: []model.Category{{ID: 1, SortKey: 0, Title: "U1", Order: "asc", Track: true, ShowInLib: true, CreatedAt: now}},
		Favourites: []model.Favourite{
			{
				MangaID: 1,
				Manga: &model.Manga{
					ID: 1, Title: "U1 Manga", URL: "/u1", PublicURL: "/u1", Rating: 1, Source: "test", CoverURL: "/u1.jpg",
				},
				CategoryID: 1, CreatedAt: now,
			},
		},
	}
	payload2 := model.FavouritesPackage{
		Categories: []model.Category{{ID: 1, SortKey: 0, Title: "U2", Order: "asc", Track: true, ShowInLib: true, CreatedAt: now}},
		Favourites: []model.Favourite{
			{
				MangaID: 2,
				Manga: &model.Manga{
					ID: 2, Title: "U2 Manga", URL: "/u2", PublicURL: "/u2", Rating: 1, Source: "test", CoverURL: "/u2.jpg",
				},
				CategoryID: 1, CreatedAt: now,
			},
		},
	}

	postFavouritesPackage(t, handler, userID1, payload1)
	postFavouritesPackage(t, handler, userID2, payload2)

	req, _ := http.NewRequest("GET", "/resource/favourites", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID1))
	rr := httptest.NewRecorder()
	handler.GetFavourites(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GetFavourites failed: %d body=%s", rr.Code, rr.Body.String())
	}

	var resp model.FavouritesPackage
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(resp.Favourites) != 1 || resp.Favourites[0].MangaID != 1 {
		t.Fatalf("expected only user1 favourites, got %+v", resp.Favourites)
	}
	if len(resp.Categories) != 1 || resp.Categories[0].Title != "U1" {
		t.Fatalf("expected only user1 categories, got %+v", resp.Categories)
	}
}

func postHistoryPackage(t *testing.T, handler *SyncHandler, userID int64, payload model.HistoryPackage) {
	t.Helper()

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/resource/history", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))
	rr := httptest.NewRecorder()
	handler.PostHistory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PostHistory failed: %d body=%s", rr.Code, rr.Body.String())
	}
}

func postFavouritesPackage(t *testing.T, handler *SyncHandler, userID int64, payload model.FavouritesPackage) {
	t.Helper()

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))
	rr := httptest.NewRecorder()
	handler.PostFavourites(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PostFavourites failed: %d body=%s", rr.Code, rr.Body.String())
	}
}
