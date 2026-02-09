set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

mysql_compose := "docker/docker-compose.mysql.yaml"
mysql_test_pattern := "Test(PostHistoryOlderUpdateDoesNotOverwriteNewer|SyncFavouritesOlderCategoryDoesNotOverwrite|SyncFavouritesTombstoneNotOverwrittenByOlderState|PostFavouritesEnsuresMissingCategoryAndManga)"
bin := "kotatsu-server"

default:
  @just --list

test:
  go test ./...

test-sqlite:
  go test ./...

test-api-sqlite:
  go test ./internal/api

build:
  go build -o {{bin}} ./cmd/server

test-race:
  go test -race ./internal/api

mysql-up:
  docker compose -f {{mysql_compose}} up -d mysql

mysql-down:
  docker compose -f {{mysql_compose}} down

mysql-logs:
  docker compose -f {{mysql_compose}} logs -f --tail=200 mysql

test-integration:
  : "${MYSQL_TEST_DSN:?Set MYSQL_TEST_DSN to a MySQL DSN}"
  go test -tags=integration ./internal/api -run '{{mysql_test_pattern}}'

test-integration-up:
  just mysql-up
  just test-integration

ci-local:
  just test-sqlite
  just test-integration
  just build
