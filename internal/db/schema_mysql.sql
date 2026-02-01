CREATE TABLE IF NOT EXISTS users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(320) NOT NULL UNIQUE,
    password_hash VARCHAR(128) NOT NULL,
    nickname VARCHAR(100),
    favourites_sync_timestamp BIGINT,
    history_sync_timestamp BIGINT,
    password_reset_token_hash VARCHAR(128) UNIQUE,
    password_reset_token_expires_at BIGINT,
    INDEX idx_users_email (email)
);

CREATE TABLE IF NOT EXISTS manga (
    id BIGINT PRIMARY KEY,
    title VARCHAR(512) NOT NULL,
    alt_title VARCHAR(512),
    url VARCHAR(1024) NOT NULL,
    public_url VARCHAR(1024) NOT NULL,
    rating DOUBLE NOT NULL,
    content_rating VARCHAR(50),
    cover_url VARCHAR(1024) NOT NULL,
    large_cover_url VARCHAR(1024),
    state VARCHAR(50),
    author VARCHAR(255),
    source VARCHAR(100) NOT NULL,
    nsfw BOOLEAN
);

CREATE TABLE IF NOT EXISTS tags (
    id BIGINT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    `key` VARCHAR(255) NOT NULL,
    source VARCHAR(100) NOT NULL,
    pinned BOOLEAN
);

CREATE TABLE IF NOT EXISTS manga_tags (
    manga_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    PRIMARY KEY (manga_id, tag_id),
    FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id),
    INDEX idx_manga_tags_tag_id (tag_id)
);

CREATE TABLE IF NOT EXISTS categories (
    id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    sort_key BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    `order` VARCHAR(50) NOT NULL,
    user_id BIGINT NOT NULL,
    track BOOLEAN NOT NULL,
    show_in_lib BOOLEAN NOT NULL,
    deleted_at BIGINT,
    PRIMARY KEY (id, user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS favourites (
    manga_id BIGINT NOT NULL,
    category_id BIGINT NOT NULL,
    sort_key BIGINT NOT NULL,
    pinned BOOLEAN NOT NULL,
    created_at BIGINT NOT NULL,
    deleted_at BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    PRIMARY KEY (manga_id, category_id, user_id),
    FOREIGN KEY (manga_id) REFERENCES manga(id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id, user_id) REFERENCES categories(id, user_id) ON DELETE CASCADE,
    INDEX idx_favourites_user_id (user_id)
);

CREATE TABLE IF NOT EXISTS history (
    manga_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    chapter_id BIGINT NOT NULL,
    page INT NOT NULL,
    scroll DOUBLE NOT NULL,
    percent DOUBLE NOT NULL,
    chapters INT NOT NULL,
    deleted_at BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    PRIMARY KEY (user_id, manga_id),
    FOREIGN KEY (manga_id) REFERENCES manga(id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_history_manga_id (manga_id)
);
