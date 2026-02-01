package model

type User struct {
	ID                        int64  `json:"id" db:"id"`
	Email                     string `json:"email" db:"email"`
	PasswordHash              string `json:"-" db:"password_hash"`
	Nickname                  *string `json:"nickname" db:"nickname"`
	FavouritesSyncTimestamp   *int64  `json:"favourites_sync_timestamp" db:"favourites_sync_timestamp"`
	HistorySyncTimestamp      *int64  `json:"history_sync_timestamp" db:"history_sync_timestamp"`
	PasswordResetTokenHash    *string `json:"-" db:"password_reset_token_hash"`
	PasswordResetTokenExpires *int64  `json:"-" db:"password_reset_token_expires_at"`
}

type Manga struct {
	ID            int64   `json:"id" db:"id"`
	Title         string  `json:"title" db:"title"`
	AltTitle      *string `json:"alt_title" db:"alt_title"`
	URL           string  `json:"url" db:"url"`
	PublicURL     string  `json:"public_url" db:"public_url"`
	Rating        float64 `json:"rating" db:"rating"`
	ContentRating *string `json:"content_rating" db:"content_rating"` // ENUM as string
	CoverURL      string  `json:"cover_url" db:"cover_url"`
	LargeCoverURL *string `json:"large_cover_url" db:"large_cover_url"`
	State         *string `json:"state" db:"state"` // ENUM as string
	Author        *string `json:"author" db:"author"`
	Source        string  `json:"source" db:"source"`
	NSFW          *bool   `json:"nsfw" db:"nsfw"`
	Tags          []Tag   `json:"tags" db:"-"`
}

type Tag struct {
	ID     int64  `json:"id" db:"id"`
	Title  string `json:"title" db:"title"`
	Key    string `json:"key" db:"key"`
	Source string `json:"source" db:"source"`
	Pinned *bool  `json:"pinned" db:"pinned"`
}

type Category struct {
	ID        int64  `json:"id" db:"id"`
	UserID    int64  `json:"user_id" db:"user_id"`
	CreatedAt int64  `json:"created_at" db:"created_at"`
	SortKey   int    `json:"sort_key" db:"sort_key"`
	Title     string `json:"title" db:"title"`
	Order     string `json:"order" db:"order"`
	Track     bool   `json:"track" db:"track"`
	ShowInLib bool   `json:"show_in_lib" db:"show_in_lib"`
	DeletedAt *int64 `json:"deleted_at" db:"deleted_at"`
}

type Favourite struct {
	MangaID    int64 `json:"manga_id" db:"manga_id"`
	Manga      *Manga `json:"manga,omitempty" db:"-"`
	CategoryID int64 `json:"category_id" db:"category_id"`
	UserID     int64 `json:"user_id" db:"user_id"`
	SortKey    int   `json:"sort_key" db:"sort_key"`
	Pinned     bool  `json:"pinned" db:"pinned"`
	CreatedAt  int64 `json:"created_at" db:"created_at"`
	DeletedAt  int64 `json:"deleted_at" db:"deleted_at"`
}

type History struct {
	MangaID   int64   `json:"manga_id" db:"manga_id"`
	Manga     *Manga  `json:"manga,omitempty" db:"-"`
	UserID    int64   `json:"user_id" db:"user_id"`
	CreatedAt int64   `json:"created_at" db:"created_at"`
	UpdatedAt int64   `json:"updated_at" db:"updated_at"`
	ChapterID int64   `json:"chapter_id" db:"chapter_id"`
	Page      int     `json:"page" db:"page"`
	Scroll    float64 `json:"scroll" db:"scroll"`
	Percent   float64 `json:"percent" db:"percent"`
	Chapters  int     `json:"chapters" db:"chapters"`
	DeletedAt int64   `json:"deleted_at" db:"deleted_at"`
}

type FavouritesPackage struct {
	Categories []Category  `json:"categories"`
	Favourites []Favourite `json:"favourites"`
	Timestamp  *int64      `json:"timestamp"`
}

type HistoryPackage struct {
	History   []History `json:"history"`
	Timestamp *int64    `json:"timestamp"`
}
