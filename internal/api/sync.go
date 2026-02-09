package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
	"github.com/theLastOfCats/kotatsu-go-server/internal/model"
)

type SyncHandler struct {
	DB *db.DB
}

func (h *SyncHandler) isMySQL() bool {
	return strings.Contains(strings.ToLower(fmt.Sprintf("%T", h.DB.Driver())), "mysql")
}

func isMySQLRetryableTxError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	// 1213: deadlock, 1205: lock wait timeout
	return mysqlErr.Number == 1213 || mysqlErr.Number == 1205
}

func (h *SyncHandler) withTxRetry(isMySQL bool, fn func(tx *sql.Tx) error) error {
	maxAttempts := 1
	if isMySQL {
		maxAttempts = 3
	}

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		tx, txErr := h.DB.Begin()
		if txErr != nil {
			return txErr
		}

		err = fn(tx)
		if err == nil {
			if commitErr := tx.Commit(); commitErr == nil {
				return nil
			} else {
				err = commitErr
			}
		}

		_ = tx.Rollback()
		if !isMySQL || !isMySQLRetryableTxError(err) || attempt == maxAttempts {
			return err
		}

		time.Sleep(time.Duration(attempt*50) * time.Millisecond)
	}

	return err
}

func (h *SyncHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	history, err := h.fetchHistory(userID)
	if err != nil {
		log.Printf("Error fetching history: %v", err)
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	timestamp := h.getTimestamp(userID, "history_sync_timestamp")
	resp := model.HistoryPackage{
		History:   history,
		Timestamp: timestamp,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (h *SyncHandler) PostHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.HistoryPackage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	isMySQL := h.isMySQL()
	now := time.Now().UnixMilli()
	err := h.withTxRetry(isMySQL, func(tx *sql.Tx) error {
		for _, item := range req.History {
			if item.Manga != nil {
				if err := upsertManga(tx, item.Manga, isMySQL); err != nil {
					return err
				}
			}
			if err := upsertHistory(tx, userID, item, isMySQL); err != nil {
				return err
			}
		}
		_, err := tx.Exec("UPDATE users SET history_sync_timestamp = ? WHERE id = ?", now, userID)
		return err
	})
	if err != nil {
		log.Printf("Error persisting history sync (user_id=%d): %v", userID, err)
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Fetch updated history to return
	history, err := h.fetchHistory(userID)
	if err != nil {
		log.Printf("Error fetching updated history: %v", err)
		// Transaction committed, but failed to fetch. Return 204 or partial error?
		// Original logic returns the package.
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	resp := model.HistoryPackage{
		History:   history,
		Timestamp: &now,
	}

	w.WriteHeader(http.StatusOK) // Explicitly set 200, though Encode likely does it
	json.NewEncoder(w).Encode(resp)
}

func (h *SyncHandler) GetFavourites(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	favourites, categories, err := h.fetchFavouritesAndCategories(userID)
	if err != nil {
		log.Printf("Error fetching favourites: %v", err)
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	timestamp := h.getTimestamp(userID, "favourites_sync_timestamp")
	resp := model.FavouritesPackage{
		Favourites: favourites,
		Categories: categories,
		Timestamp:  timestamp,
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *SyncHandler) PostFavourites(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		log.Println("PostFavourites: Failed to get userID from context")
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("PostFavourites: Starting for user %d", userID)

	var req model.FavouritesPackage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	isMySQL := h.isMySQL()
	now := time.Now().UnixMilli()

	// Stable lock order reduces deadlock probability on concurrent sync requests.
	sort.Slice(req.Categories, func(i, j int) bool {
		return req.Categories[i].ID < req.Categories[j].ID
	})
	sort.Slice(req.Favourites, func(i, j int) bool {
		if req.Favourites[i].MangaID != req.Favourites[j].MangaID {
			return req.Favourites[i].MangaID < req.Favourites[j].MangaID
		}
		return req.Favourites[i].CategoryID < req.Favourites[j].CategoryID
	})

	err := h.withTxRetry(isMySQL, func(tx *sql.Tx) error {
		for _, category := range req.Categories {
			if err := upsertCategory(tx, userID, category, isMySQL); err != nil {
				return err
			}
		}

		for _, fav := range req.Favourites {
			log.Printf("Processing favourite: manga_id=%d, category_id=%d, has_manga_object=%v", fav.MangaID, fav.CategoryID, fav.Manga != nil)

			if fav.Manga != nil {
				if err := upsertManga(tx, fav.Manga, isMySQL); err != nil {
					return err
				}
				log.Printf("Successfully upserted manga %d", fav.MangaID)
			} else if fav.MangaID != 0 {
				// Ensure manga record exists for foreign key constraint
				if err := ensureMangaExists(tx, fav.MangaID); err != nil {
					return err
				}
				log.Printf("Ensured manga %d exists", fav.MangaID)
			}

			// Ensure category exists before inserting favourite
			// This handles race conditions when multiple devices sync simultaneously
			if err := ensureCategoryExists(tx, fav.CategoryID, userID); err != nil {
				return err
			}
			log.Printf("Ensured category %d exists for user %d", fav.CategoryID, userID)

			if err := upsertFavourite(tx, userID, fav, isMySQL); err != nil {
				return err
			}
			log.Printf("Successfully upserted favourite: manga_id=%d, category_id=%d", fav.MangaID, fav.CategoryID)
		}

		_, err := tx.Exec("UPDATE users SET favourites_sync_timestamp = ? WHERE id = ?", now, userID)
		return err
	})
	if err != nil {
		log.Printf("Error persisting favourites sync (user_id=%d): %v", userID, err)
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	favourites, categories, err := h.fetchFavouritesAndCategories(userID)
	if err != nil {
		log.Printf("Error fetching updated favourites: %v", err)
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	resp := model.FavouritesPackage{
		Favourites: favourites,
		Categories: categories,
		Timestamp:  &now,
	}

	json.NewEncoder(w).Encode(resp)
}

// Helpers

func (h *SyncHandler) getTimestamp(userID int64, column string) *int64 {
	var timestamp sql.NullInt64
	query := "SELECT " + column + " FROM users WHERE id = ?"
	h.DB.QueryRow(query, userID).Scan(&timestamp)
	if timestamp.Valid {
		return &timestamp.Int64
	}
	return nil
}

func upsertManga(tx *sql.Tx, manga *model.Manga, isMySQL bool) error {
	query := `INSERT INTO manga (id, title, alt_title, url, public_url, rating, content_rating, cover_url, large_cover_url, state, author, source, nsfw)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
	title=excluded.title, alt_title=excluded.alt_title, url=excluded.url, public_url=excluded.public_url, rating=excluded.rating, content_rating=excluded.content_rating,
	cover_url=excluded.cover_url, large_cover_url=excluded.large_cover_url, state=excluded.state, author=excluded.author, source=excluded.source, nsfw=excluded.nsfw`
	if isMySQL {
		query = `INSERT INTO manga (id, title, alt_title, url, public_url, rating, content_rating, cover_url, large_cover_url, state, author, source, nsfw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		title=VALUES(title), alt_title=VALUES(alt_title), url=VALUES(url), public_url=VALUES(public_url), rating=VALUES(rating), content_rating=VALUES(content_rating),
		cover_url=VALUES(cover_url), large_cover_url=VALUES(large_cover_url), state=VALUES(state), author=VALUES(author), source=VALUES(source), nsfw=VALUES(nsfw)`
	}

	_, err := tx.Exec(query, manga.ID, manga.Title, manga.AltTitle, manga.URL, manga.PublicURL, manga.Rating, manga.ContentRating, manga.CoverURL, manga.LargeCoverURL, manga.State, manga.Author, manga.Source, manga.NSFW)
	if err != nil {
		return err
	}

	for _, tag := range manga.Tags {
		if err := upsertTag(tx, tag, isMySQL); err != nil {
			return err
		}
		tagLinkQuery := "INSERT OR IGNORE INTO manga_tags (manga_id, tag_id) VALUES (?, ?)"
		if isMySQL {
			tagLinkQuery = "INSERT IGNORE INTO manga_tags (manga_id, tag_id) VALUES (?, ?)"
		}
		if _, err := tx.Exec(tagLinkQuery, manga.ID, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

// ensureMangaExists inserts a placeholder manga record if it doesn't exist
// This is needed when the app sends manga_id without manga object (for already-synced manga)
func ensureMangaExists(tx *sql.Tx, mangaID int64) error {
	// Check if manga exists
	var exists bool
	err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?)", mangaID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		// Insert placeholder manga record to satisfy foreign key constraint
		query := `INSERT INTO manga (id, title, alt_title, url, public_url, rating, content_rating, cover_url, large_cover_url, state, author, source, nsfw)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := tx.Exec(query, mangaID, "", "", "", "", -1.0, "", "", "", "", "", "", false)
		return err
	}
	return nil
}

// ensureCategoryExists inserts a placeholder category if it doesn't exist
// This handles cases where favourites reference categories that haven't been synced yet
func ensureCategoryExists(tx *sql.Tx, categoryID int64, userID int64) error {
	// Check if category exists
	var exists bool
	err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE id = ? AND user_id = ?)", categoryID, userID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		// Insert placeholder category to satisfy foreign key constraint
		query := "INSERT INTO categories (id, user_id, created_at, sort_key, title, `order`, track, show_in_lib, deleted_at)\n" +
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
		_, err := tx.Exec(query, categoryID, userID, 0, 0, "Unknown", "NEWEST", true, true, 0)
		return err
	}
	return nil
}

func upsertTag(tx *sql.Tx, tag model.Tag, isMySQL bool) error {
	query := "INSERT INTO tags (id, title, `key`, source, pinned) VALUES (?, ?, ?, ?, ?)\n" +
		"ON CONFLICT(id) DO UPDATE SET title=excluded.title, `key`=excluded.`key`, source=excluded.source, pinned=excluded.pinned"
	if isMySQL {
		query = "INSERT INTO tags (id, title, `key`, source, pinned) VALUES (?, ?, ?, ?, ?)\n" +
			"ON DUPLICATE KEY UPDATE title=VALUES(title), `key`=VALUES(`key`), source=VALUES(source), pinned=VALUES(pinned)"
	}
	_, err := tx.Exec(query, tag.ID, tag.Title, tag.Key, tag.Source, tag.Pinned)
	return err
}

func upsertHistory(tx *sql.Tx, userID int64, history model.History, isMySQL bool) error {
	query := `INSERT INTO history (manga_id, user_id, created_at, updated_at, chapter_id, page, scroll, percent, chapters, deleted_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, manga_id) DO UPDATE SET
	created_at=excluded.created_at, updated_at=excluded.updated_at, chapter_id=excluded.chapter_id, page=excluded.page,
	scroll=excluded.scroll, percent=excluded.percent, chapters=excluded.chapters, deleted_at=excluded.deleted_at
	WHERE excluded.updated_at >= history.updated_at`
	if isMySQL {
		query = `INSERT INTO history (manga_id, user_id, created_at, updated_at, chapter_id, page, scroll, percent, chapters, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		created_at=IF(VALUES(updated_at) >= updated_at, VALUES(created_at), created_at),
		chapter_id=IF(VALUES(updated_at) >= updated_at, VALUES(chapter_id), chapter_id),
		page=IF(VALUES(updated_at) >= updated_at, VALUES(page), page),
		scroll=IF(VALUES(updated_at) >= updated_at, VALUES(scroll), scroll),
		percent=IF(VALUES(updated_at) >= updated_at, VALUES(percent), percent),
		chapters=IF(VALUES(updated_at) >= updated_at, VALUES(chapters), chapters),
		deleted_at=IF(VALUES(updated_at) >= updated_at, VALUES(deleted_at), deleted_at),
		updated_at=IF(VALUES(updated_at) >= updated_at, VALUES(updated_at), updated_at)`
	}
	_, err := tx.Exec(query, history.MangaID, userID, history.CreatedAt, history.UpdatedAt, history.ChapterID, history.Page, history.Scroll, history.Percent, history.Chapters, history.DeletedAt)
	return err
}

func upsertCategory(tx *sql.Tx, userID int64, cat model.Category, isMySQL bool) error {
	query := "INSERT INTO categories (id, user_id, created_at, sort_key, title, `order`, track, show_in_lib, deleted_at)\n" +
		`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, user_id) DO UPDATE SET
		created_at=excluded.created_at, sort_key=excluded.sort_key, title=excluded.title, ` + "`order`=excluded.`order`," + `
		track=excluded.track, show_in_lib=excluded.show_in_lib, deleted_at=excluded.deleted_at
		WHERE excluded.created_at > categories.created_at
		OR (excluded.created_at = categories.created_at AND COALESCE(excluded.deleted_at, 0) >= COALESCE(categories.deleted_at, 0))`
	if isMySQL {
		query = `INSERT INTO categories (id, user_id, created_at, sort_key, title, ` + "`order`" + `, track, show_in_lib, deleted_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE
    sort_key=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND COALESCE(VALUES(deleted_at), 0) >= COALESCE(deleted_at, 0)), VALUES(sort_key), sort_key),
    title=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND COALESCE(VALUES(deleted_at), 0) >= COALESCE(deleted_at, 0)), VALUES(title), title),
    ` + "`order`" + `=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND COALESCE(VALUES(deleted_at), 0) >= COALESCE(deleted_at, 0)), VALUES(` + "`order`" + `), ` + "`order`" + `),
    track=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND COALESCE(VALUES(deleted_at), 0) >= COALESCE(deleted_at, 0)), VALUES(track), track),
    show_in_lib=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND COALESCE(VALUES(deleted_at), 0) >= COALESCE(deleted_at, 0)), VALUES(show_in_lib), show_in_lib),
    deleted_at=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND COALESCE(VALUES(deleted_at), 0) >= COALESCE(deleted_at, 0)), VALUES(deleted_at), deleted_at),
    created_at=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND COALESCE(VALUES(deleted_at), 0) >= COALESCE(deleted_at, 0)), VALUES(created_at), created_at)`
	}
	_, err := tx.Exec(query, cat.ID, userID, cat.CreatedAt, cat.SortKey, cat.Title, cat.Order, cat.Track, cat.ShowInLib, cat.DeletedAt)
	return err
}

func upsertFavourite(tx *sql.Tx, userID int64, fav model.Favourite, isMySQL bool) error {
	query := `INSERT INTO favourites (manga_id, category_id, user_id, sort_key, pinned, created_at, deleted_at)
    VALUES (?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(manga_id, category_id, user_id) DO UPDATE SET
    category_id=excluded.category_id,
    sort_key=excluded.sort_key, pinned=excluded.pinned,
    created_at=excluded.created_at, deleted_at=excluded.deleted_at
    WHERE excluded.created_at > favourites.created_at OR (excluded.created_at = favourites.created_at AND excluded.deleted_at > favourites.deleted_at)`
	if isMySQL {
		query = `INSERT INTO favourites (manga_id, category_id, user_id, sort_key, pinned, created_at, deleted_at)
    VALUES (?, ?, ?, ?, ?, ?, ?)
    ON DUPLICATE KEY UPDATE
    category_id=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND VALUES(deleted_at) > deleted_at), VALUES(category_id), category_id),
    sort_key=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND VALUES(deleted_at) > deleted_at), VALUES(sort_key), sort_key),
    pinned=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND VALUES(deleted_at) > deleted_at), VALUES(pinned), pinned),
    deleted_at=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND VALUES(deleted_at) > deleted_at), VALUES(deleted_at), deleted_at),
    created_at=IF(VALUES(created_at) > created_at OR (VALUES(created_at) = created_at AND VALUES(deleted_at) > deleted_at), VALUES(created_at), created_at)`
	}
	_, err := tx.Exec(query, fav.MangaID, fav.CategoryID, userID, fav.SortKey, fav.Pinned, fav.CreatedAt, fav.DeletedAt)
	return err
}

func (h *SyncHandler) fetchHistory(userID int64) ([]model.History, error) {
	rows, err := h.DB.Query(`SELECT manga_id, created_at, updated_at, chapter_id, page, scroll, percent, chapters, deleted_at FROM history WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []model.History
	for rows.Next() {
		var hItem model.History
		hItem.UserID = userID
		if err := rows.Scan(&hItem.MangaID, &hItem.CreatedAt, &hItem.UpdatedAt, &hItem.ChapterID, &hItem.Page, &hItem.Scroll, &hItem.Percent, &hItem.Chapters, &hItem.DeletedAt); err != nil {
			return nil, err
		}

		manga, err := h.fetchManga(hItem.MangaID)
		if err != nil {
			return nil, err
		}
		hItem.Manga = manga

		history = append(history, hItem)
	}
	return history, nil
}

func (h *SyncHandler) fetchFavouritesAndCategories(userID int64) ([]model.Favourite, []model.Category, error) {
	// Categories
	rows, err := h.DB.Query("SELECT id, created_at, sort_key, title, `order`, track, show_in_lib, deleted_at FROM categories WHERE user_id = ?", userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var cat model.Category
		cat.UserID = userID
		if err := rows.Scan(&cat.ID, &cat.CreatedAt, &cat.SortKey, &cat.Title, &cat.Order, &cat.Track, &cat.ShowInLib, &cat.DeletedAt); err != nil {
			return nil, nil, err
		}
		categories = append(categories, cat)
	}

	// Favourites
	favRows, err := h.DB.Query(`SELECT manga_id, category_id, sort_key, pinned, created_at, deleted_at FROM favourites WHERE user_id = ?`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer favRows.Close()

	var favourites []model.Favourite
	for favRows.Next() {
		var fav model.Favourite
		fav.UserID = userID
		if err := favRows.Scan(&fav.MangaID, &fav.CategoryID, &fav.SortKey, &fav.Pinned, &fav.CreatedAt, &fav.DeletedAt); err != nil {
			return nil, nil, err
		}

		manga, err := h.fetchManga(fav.MangaID)
		if err != nil {
			return nil, nil, err
		}
		fav.Manga = manga

		favourites = append(favourites, fav)
	}

	return favourites, categories, nil
}

func (h *SyncHandler) fetchManga(mangaID int64) (*model.Manga, error) {
	row := h.DB.QueryRow(`SELECT id, title, alt_title, url, public_url, rating, content_rating, cover_url, large_cover_url, state, author, source, nsfw FROM manga WHERE id = ?`, mangaID)
	var m model.Manga
	if err := row.Scan(&m.ID, &m.Title, &m.AltTitle, &m.URL, &m.PublicURL, &m.Rating, &m.ContentRating, &m.CoverURL, &m.LargeCoverURL, &m.State, &m.Author, &m.Source, &m.NSFW); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	tags, err := h.fetchTags(m.ID)
	if err != nil {
		return nil, err
	}
	m.Tags = tags

	return &m, nil
}

func (h *SyncHandler) fetchTags(mangaID int64) ([]model.Tag, error) {
	rows, err := h.DB.Query("SELECT t.id, t.title, t.`key`, t.source, t.pinned "+
		`
                             FROM tags t 
                             JOIN manga_tags mt ON t.id = mt.tag_id 
                             WHERE mt.manga_id = ?`, mangaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Title, &t.Key, &t.Source, &t.Pinned); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}
