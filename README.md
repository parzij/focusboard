# FocusBoard

Персональное приложение для работы с задачами. В MVP входят личная Kanban-доска, сохранение порядка, профиль и статистика по первому закрытию каждой задачи.

На этапе 1 Backend подключён к PostgreSQL и отдаёт JSON-пробы состояния. Frontend запускается отдельно и показывает заголовок приложения. Порядок работ описан в [дорожной карте](docs/ROADMAP.md).

## Запуск Backend и PostgreSQL

Понадобится Docker с Compose. Из корня проекта:

```sh
cp .env.example .env
docker compose config --quiet
docker compose up --build -d
docker compose ps -a
curl -i http://127.0.0.1:8080/api/v1/health
curl -i http://127.0.0.1:8080/api/v1/ready
```

PostgreSQL доступна контейнерам на `db:5432`. Compose ждёт её healthcheck, запускает Goose и стартует API после успешного завершения миграций. API опубликован только на loopback. `API_PORT` задаёт порт на хосте, `API_CONTAINER_PORT` задаёт порт внутри сети Compose. Состояние БД переживает перезапуск благодаря именованному тому.

Пример в `.env.example` предназначен для локальной разработки. При смене пароля обнови `POSTGRES_PASSWORD` и пароль в `DATABASE_URL`. Специальные символы в URL нужно кодировать. В уже инициализированной PostgreSQL смена переменной сама по себе пароль роли не меняет. Для внешней БД используй проверку TLS по требованиям окружения.

Для запуска Go вне Compose укажи URL доступной с хоста PostgreSQL с применёнными миграциями. Go читает переменные процесса. `HTTP_ADDR` по умолчанию равен `127.0.0.1:8080`.

```sh
cd backend
go run ./cmd/migrate up
go run ./cmd/api
```

Таймауты HTTP и БД, параметры пула и уровень логов заданы в [шаблоне среды](.env.example). `DB_CHECK_TIMEOUT` должен быть меньше таймаута записи HTTP, а `DB_POOL_MIN_CONNS` не должен превышать `DB_POOL_MAX_CONNS`. При увеличении таймаута graceful shutdown задай больший `API_STOP_GRACE_PERIOD`. Команды проверки миграций и резервного копирования описаны в [документации БД](docs/DATABASE.md).

## Запуск Frontend

Понадобятся Node.js 26.13.1 и pnpm 11.6.0. Из корня проекта:

```sh
cd frontend
npm install --global pnpm@11.6.0
pnpm install --frozen-lockfile
pnpm dev
```

Открой адрес из вывода Vite.

## Проверки

Из корня проекта. Для локальной сборки Backend используется Go 1.27.4, как в CI.

```sh
(cd backend && test -z "$(gofmt -l .)" && go mod verify && go vet ./... && go test -race ./... && go build ./...)
(cd frontend && pnpm install --frozen-lockfile && pnpm lint && pnpm format:check && pnpm typecheck && pnpm test && pnpm build)
bash scripts/test-compose.sh
```

Интеграционные тесты Go запускаются при заданном `FOCUSBOARD_TEST_DATABASE_URL` для отдельной базы `focusboard_test`. [GitHub Actions](.github/workflows/ci.yml) поднимает её автоматически. Compose smoke создаёт отдельный тестовый проект, проверяет порядок запуска, потерю и восстановление связи с PostgreSQL и сохранность данных. После прогона тестовые контейнеры останавливаются, тестовый том сохраняется.

## Документация

- [Техническое задание](SPECIFICATION.md) и [макеты](docs/mockups/)
- [Инструкции Codex](AGENTS.md) и [дорожная карта](docs/ROADMAP.md)
- [Архитектура](docs/ARCHITECTURE.md) и [решения](docs/DECISIONS.md)
- [База данных](docs/DATABASE.md), [REST API](docs/API.md) и [дизайн-система](docs/DESIGN_SYSTEM.md)
- [История изменений](docs/CHANGELOG.md)
