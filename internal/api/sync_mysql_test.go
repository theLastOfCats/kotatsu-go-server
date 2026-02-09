//go:build integration

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

func TestPostHistoryOlderUpdateDoesNotOverwriteNewerMySQL(t *testing.T) {
	database := testutil.SetupMySQLTestDB(t)

	res, err := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "history-order-mysql@example.com", "hash")
	if err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
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

func TestSyncFavouritesOlderCategoryDoesNotOverwriteMySQL(t *testing.T) {
	database := testutil.SetupMySQLTestDB(t)

	res, err := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "multi-device-cat-mysql@example.com", "hash")
	if err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
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

func TestSyncFavouritesTombstoneNotOverwrittenByOlderStateMySQL(t *testing.T) {
	database := testutil.SetupMySQLTestDB(t)

	res, err := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "multi-device-fav-mysql@example.com", "hash")
	if err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
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

func TestPostFavouritesEnsuresMissingCategoryAndMangaMySQL(t *testing.T) {
	database := testutil.SetupMySQLTestDB(t)

	res, err := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "placeholder-mysql@example.com", "hash")
	if err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
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

func TestMySQLIntegrationSmokeUsesCurrentTime(t *testing.T) {
	database := testutil.SetupMySQLTestDB(t)
	handler := &SyncHandler{DB: database}

	res, err := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", "smoke-mysql@example.com", "hash")
	if err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
	userID, _ := res.LastInsertId()

	now := time.Now().UnixMilli()
	payload := model.FavouritesPackage{
		Categories: []model.Category{
			{ID: 1, SortKey: 0, Title: "Now", Order: "NEWEST", Track: true, ShowInLib: true, CreatedAt: now},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/resource/favourites", bytes.NewBuffer(body))
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))
	rr := httptest.NewRecorder()
	handler.PostFavourites(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("smoke PostFavourites failed: %d body: %s", rr.Code, rr.Body.String())
	}
}

