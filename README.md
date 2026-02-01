# Kotatsu Synchronization Server

[Kotatsu](https://github.com/Kotatsu-Redo/Kotatsu) is a free and open source manga reader for Android platform. This is the Go implementation of the synchronization server, designed to be lightweight, performant, and feature-complete.

## Features

- **Synchronization**: Sync favorites, categories, and reading history across devices.
- **Authentication**: JWT-based auth with user registration and login.
- **Password Reset**: Full flow including email dispatch and deeplinking.
- **Health Check**: `/` endpoint for uptime monitoring.
- **Database**: 
    - **Local**: SQLite with production optimizations (WAL, Foreign Keys).
    - **Remote**: MySQL support for easy migration from kotatsu-syncserver.
- **Mail**: SMTP support for transactional emails.

## Installation

> [!TIP]
> You can find the entire list of environment variables in the `.env.example` file.

### Docker

#### Build image:

```shell
docker build . -t kotatsu-go-server
```

#### Run container:

```shell
docker run -d -p 8080:8080 \
  -e JWT_SECRET=your_secret \
  --restart always \
  --name kotatsu-sync kotatsu-go-server
```

### Manual (Go)

Requirements: Go 1.24+

```shell
git clone https://github.com/theLastOfCats/kotatsu-go-server.git
cd kotatsu-go-server
go build -o kotatsu-server ./cmd/server
```

Run the server:

```shell
export JWT_SECRET=your_secret
./kotatsu-server
```

## Configuration

The server is configured using environment variables or a `.env` file.

| Variable | Description | Default |
|---|---|---|
| `JWT_SECRET` | **Required.** Secret key for signing JWT tokens. | None |
| `DB_PATH` | Path to SQLite file OR MySQL DSN (see below). | `data/kotatsu.db` |
| `PORT` | Port to listen on. | `8080` |
| `BASE_URL` | Base URL for generating deeplinks (e.g., in emails). | `http://localhost:8080` |

### Database Configuration

**SQLite (Local)**:
```bash
DB_PATH=data/kotatsu.db
```

**MySQL (Remote)** - Compatible with original kotatsu-syncserver:
```bash
DB_PATH="user:password@tcp(hostname:3306)/database_name?parseTime=true"
```

The server automatically detects the database type based on the DSN format.

### Mail Configuration (SMTP)

To enable password reset emails, configure an SMTP provider:

| Variable | Description | Example |
|---|---|---|
| `MAIL_PROVIDER` | `smtp` or `console` (logs to stdout). | `smtp` |
| `SMTP_HOST` | SMTP Server Host. | `smtp.gmail.com` |
| `SMTP_PORT` | SMTP Server Port. | `587` |
| `SMTP_USER` | SMTP Username. | `user@example.com` |
| `SMTP_PASSWORD` | SMTP Password. | `secret` |
| `SMTP_FROM` | Sender email address. | `noreply@kotatsu.app` |

### Remote Database (Turso)

To use a remote LibSQL/Turso database instead of a local file:

```shell
export DB_PATH="libsql://your-db-name.turso.io?authToken=ey..."
```

## Testing

This project includes a comprehensive test suite covering Auth, Sync, and User flows.
It uses an in-memory SQLite database and mock mailer, so no external dependencies are needed to run tests.

```shell
go test -v ./...
```

To see coverage:

```shell
go test -coverprofile=coverage.out ./internal/api
go tool cover -func=coverage.out
```

## API Endpoints

### Public
- `GET /` - Health check ("Alive")
- `POST /auth/login` - Login (returns JWT)
- `POST /auth/register` - Register (returns JWT)
- `POST /forgot-password` - Request password reset
- `POST /reset-password` - Reset password with token
- `GET /deeplink/reset-password` - HTML page for password reset

### Protected (Bearer Token)
- `GET /me` - Get current user info
- `GET/POST /resource/history` - Sync reading history
- `GET/POST /resource/favourites` - Sync favourites and categories

## License

[![MIT License](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
