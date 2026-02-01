# Kotatsu Synchronization Server

[Kotatsu](https://github.com/Kotatsu-Redo/Kotatsu) is a free and open source manga reader for Android platform. Supports a lot of online catalogues on different languages with filters and search, offline reading from local storage, favourites, bookmarks, new chapters notifications and more features.

### What is synchronization?

Synchronization is needed to store your collection of favorites, history and categories and have remote access to them. On a synchronized device, you can restore your manga collection in real time without loss. It also supports working across multiple devices. It is convenient for those who use several devices.

### How does synchronization work?

- An account is created and configured in the application where it will store data;
- Synchronization starts. The data selected by the user is saved on the service and stored there under protection;
- Another device connects and syncs with the service;
- The uploaded data appears on the device connected to the account.

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

### Docker compose

#### Clone the repository:

```shell
git clone https://github.com/theLastOfCats/kotatsu-go-server.git \
  && cd kotatsu-go-server
```

#### Specify your settings (optional)

You can override settings via the `.env` file.

```shell
cp .env.example .env
```

#### Start services:

```shell
docker compose up -d
```

### Manual (Binary)

Download the latest release from the [Releases](https://github.com/theLastOfCats/kotatsu-go-server/releases) page.

Run the server:

```shell
export JWT_SECRET=your_secret
./kotatsu-syncserver
```

### Manual (Go)

Requirements:

1. Go 1.24+

Commands:

```shell
git clone https://github.com/theLastOfCats/kotatsu-go-server.git \
  && cd kotatsu-go-server \
  && go build -o kotatsu-server ./cmd/server
```

Run the server:

```shell
export JWT_SECRET=your_secret
./kotatsu-syncserver
```

## Configuration

The server is configured using environment variables.

| Variable | Description | Default |
|---|---|---|
| `JWT_SECRET` | **Required.** Secret key for signing JWT tokens. | None |
| `DB_PATH` | Path to the SQLite database file. | `data/kotatsu.db` |
| `PORT` | Port to listen on. | `8080` |

## FAQ

### What data can be synchronized?

- Favorites (with categories);
- History.

### How do I sync my data?

Go to `Options -> Settings -> Services`, then select **Synchronization**. Enter your email address (even if you have not registered in the synchronization system, the authorization screen also acts as a registration screen), then come up with and enter a password.

After the authorization/registration process, you will return back to the **Content** screen. To set up synchronization, select **Synchronization** again, and then you will go to system sync settings. Choose what you want to sync, history, favorites or all together, after which automatic synchronization to our server will begin.

### Can I use a synchronization server on my hosting?

Yes, you can use your synchronization server in the application by specifying its address (`Options -> Settings -> Services -> Synchronization settings -> Server address`). Instructions for deploying the server are above.

## License

[![MIT License](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
