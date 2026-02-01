package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
	"github.com/theLastOfCats/kotatsu-go-server/internal/model"
)

type SyncHandler struct {
	DB *db.DB
}

func (h *SyncHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	history, err := h.fetchHistory(userID)
	if err != nil {
		log.Printf("Error fetching history: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.HistoryPackage
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	for _, item := range req.History {
		if item.Manga != nil {
			if err := upsertManga(tx, item.Manga); err != nil {
				log.Printf("Error upserting manga: %v", err)
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
		}
		if err := upsertHistory(tx, userID, item); err != nil {
			log.Printf("Error upserting history: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}

	now := time.Now().UnixMilli()
	if _, err := tx.Exec("UPDATE users SET history_sync_timestamp = ? WHERE id = ?", now, userID); err != nil {
		log.Printf("Error updating timestamp: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

    if err := tx.Commit(); err != nil {
         log.Printf("Error committing tx: %v", err)
         http.Error(w, "Database error", http.StatusInternalServerError)
         return
    }

	// Fetch updated history to return
	history, err := h.fetchHistory(userID)
	if err != nil {
         log.Printf("Error fetching updated history: %v", err)
		// Transaction committed, but failed to fetch. Return 204 or partial error?
        // Original logic returns the package.
		http.Error(w, "Database error", http.StatusInternalServerError)
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
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    favourites, categories, err := h.fetchFavouritesAndCategories(userID)
    if err != nil {
        log.Printf("Error fetching favourites: %v", err)
        http.Error(w, "Database error", http.StatusInternalServerError)
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
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req model.FavouritesPackage
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    tx, err := h.DB.Begin()
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }
    defer tx.Rollback()

    for _, category := range req.Categories {
        if err := upsertCategory(tx, userID, category); err != nil {
             log.Printf("Error upserting category: %v", err)
             http.Error(w, "Database error", http.StatusInternalServerError)
             return
        }
    }

    for _, fav := range req.Favourites {
        if fav.Manga != nil {
            if err := upsertManga(tx, fav.Manga); err != nil {
                 log.Printf("Error upserting manga for fav: %v", err)
                 http.Error(w, "Database error", http.StatusInternalServerError)
                 return
            }
        }
        if err := upsertFavourite(tx, userID, fav); err != nil {
             log.Printf("Error upserting favourite: %v", err)
             http.Error(w, "Database error", http.StatusInternalServerError)
             return
        }
    }

    now := time.Now().UnixMilli()
    if _, err := tx.Exec("UPDATE users SET favourites_sync_timestamp = ? WHERE id = ?", now, userID); err != nil {
         log.Printf("Error updating timestamp: %v", err)
         http.Error(w, "Database error", http.StatusInternalServerError)
         return
    }

    if err := tx.Commit(); err != nil {
        log.Printf("Error committing tx: %v", err)
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    favourites, categories, err := h.fetchFavouritesAndCategories(userID)
    if err != nil {
         log.Printf("Error fetching updated favourites: %v", err)
         http.Error(w, "Database error", http.StatusInternalServerError)
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

func upsertManga(tx *sql.Tx, manga *model.Manga) error {
	query := `INSERT INTO manga (id, title, alt_title, url, public_url, rating, content_rating, cover_url, large_cover_url, state, author, source, nsfw)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
	title=excluded.title, alt_title=excluded.alt_title, url=excluded.url, public_url=excluded.public_url, rating=excluded.rating, content_rating=excluded.content_rating,
	cover_url=excluded.cover_url, large_cover_url=excluded.large_cover_url, state=excluded.state, author=excluded.author, source=excluded.source, nsfw=excluded.nsfw`
	
	_, err := tx.Exec(query, manga.ID, manga.Title, manga.AltTitle, manga.URL, manga.PublicURL, manga.Rating, manga.ContentRating, manga.CoverURL, manga.LargeCoverURL, manga.State, manga.Author, manga.Source, manga.NSFW)
	if err != nil {
		return err
	}

    for _, tag := range manga.Tags {
        if err := upsertTag(tx, tag); err != nil {
            return err
        }
        if _, err := tx.Exec("INSERT OR IGNORE INTO manga_tags (manga_id, tag_id) VALUES (?, ?)", manga.ID, tag.ID); err != nil {
            return err
        }
    }
    return nil
}

func upsertTag(tx *sql.Tx, tag model.Tag) error {
    query := `INSERT INTO tags (id, title, "key", source, pinned) VALUES (?, ?, ?, ?, ?)
    ON CONFLICT(id) DO UPDATE SET title=excluded.title, "key"=excluded.key, source=excluded.source, pinned=excluded.pinned`
    _, err := tx.Exec(query, tag.ID, tag.Title, tag.Key, tag.Source, tag.Pinned)
    return err
}

func upsertHistory(tx *sql.Tx, userID int64, history model.History) error {
	query := `INSERT INTO history (manga_id, user_id, created_at, updated_at, chapter_id, page, scroll, percent, chapters, deleted_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, manga_id) DO UPDATE SET
	created_at=excluded.created_at, updated_at=excluded.updated_at, chapter_id=excluded.chapter_id, page=excluded.page,
	scroll=excluded.scroll, percent=excluded.percent, chapters=excluded.chapters, deleted_at=excluded.deleted_at`
	_, err := tx.Exec(query, history.MangaID, userID, history.CreatedAt, history.UpdatedAt, history.ChapterID, history.Page, history.Scroll, history.Percent, history.Chapters, history.DeletedAt)
	return err
}

func upsertCategory(tx *sql.Tx, userID int64, cat model.Category) error {
    query := `INSERT INTO categories (id, user_id, created_at, sort_key, title, "order", track, show_in_lib, deleted_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(id, user_id) DO UPDATE SET
    created_at=excluded.created_at, sort_key=excluded.sort_key, title=excluded.title, "order"=excluded.order,
    track=excluded.track, show_in_lib=excluded.show_in_lib, deleted_at=excluded.deleted_at`
    _, err := tx.Exec(query, cat.ID, userID, cat.CreatedAt, cat.SortKey, cat.Title, cat.Order, cat.Track, cat.ShowInLib, cat.DeletedAt)
    return err
}

func upsertFavourite(tx *sql.Tx, userID int64, fav model.Favourite) error {
    query := `INSERT INTO favourites (manga_id, category_id, user_id, sort_key, pinned, created_at, deleted_at)
    VALUES (?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(manga_id, category_id, user_id) DO UPDATE SET
    sort_key=excluded.sort_key, pinned=excluded.pinned, created_at=excluded.created_at, deleted_at=excluded.deleted_at`
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
    rows, err := h.DB.Query(`SELECT id, created_at, sort_key, title, "order", track, show_in_lib, deleted_at FROM categories WHERE user_id = ?`, userID)
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
    rows, err := h.DB.Query(`SELECT t.id, t.title, t.key, t.source, t.pinned 
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
