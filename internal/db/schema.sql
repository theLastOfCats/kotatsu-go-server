CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    nickname TEXT,
    favourites_sync_timestamp INTEGER,
    history_sync_timestamp INTEGER,
    password_reset_token_hash TEXT UNIQUE,
    password_reset_token_expires_at INTEGER
);

CREATE TABLE IF NOT EXISTS manga (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    alt_title TEXT,
    url TEXT NOT NULL,
    public_url TEXT NOT NULL,
    rating REAL NOT NULL,
    content_rating TEXT,
    cover_url TEXT NOT NULL,
    large_cover_url TEXT,
    state TEXT,
    author TEXT,
    source TEXT NOT NULL,
    nsfw BOOLEAN
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    key TEXT NOT NULL,
    source TEXT NOT NULL,
    pinned BOOLEAN
);

CREATE TABLE IF NOT EXISTS manga_tags (
    manga_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (manga_id, tag_id),
    FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id)
);

CREATE TABLE IF NOT EXISTS categories (
    id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    sort_key INTEGER NOT NULL,
    title TEXT NOT NULL,
    `order` TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    track BOOLEAN NOT NULL,
    show_in_lib BOOLEAN NOT NULL,
    deleted_at INTEGER,
    PRIMARY KEY (id, user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS favourites (
    manga_id INTEGER NOT NULL,
    category_id INTEGER NOT NULL,
    sort_key INTEGER NOT NULL,
    pinned BOOLEAN NOT NULL,
    created_at INTEGER NOT NULL,
    deleted_at INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    PRIMARY KEY (manga_id, category_id, user_id),
    FOREIGN KEY (manga_id) REFERENCES manga(id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id, user_id) REFERENCES categories(id, user_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS history (
    manga_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    chapter_id INTEGER NOT NULL,
    page INTEGER NOT NULL,
    scroll REAL NOT NULL,
    percent REAL NOT NULL,
    chapters INTEGER NOT NULL,
    deleted_at INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, manga_id),
    FOREIGN KEY (manga_id) REFERENCES manga(id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_manga_tags_tag_id ON manga_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_favourites_user_id ON favourites(user_id);
CREATE INDEX IF NOT EXISTS idx_history_manga_id ON history(manga_id);
