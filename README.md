# FocusBoard

Персональное приложение для работы с задачами. В MVP входят личная Kanban-доска, сохранение порядка, профиль и статистика по первому закрытию каждой задачи.

Backend поддерживает регистрацию, сессии, профиль, API задач с историей статусов и статистику первых закрытий по календарным дням и неделям. Frontend запускается отдельно и показывает заголовок приложения. Порядок работ описан в [дорожной карте](docs/ROADMAP.md).

## Запуск Backend и PostgreSQL

Понадобится Docker с Compose. Из корня проекта:

```sh
cp .env.example .env
openssl rand -base64 32
# Paste the generated value into JWT_SECRET in .env.
docker compose config --quiet
docker compose up --build -d
docker compose ps -a
curl -i http://127.0.0.1:8080/api/v1/health
curl -i http://127.0.0.1:8080/api/v1/ready
```

PostgreSQL доступна контейнерам на `db:5432`. Compose ждёт её healthcheck, запускает Goose и стартует API после успешного завершения миграций. API опубликован только на loopback. `API_PORT` задаёт порт на хосте, `API_CONTAINER_PORT` задаёт порт внутри сети Compose. Состояние БД переживает перезапуск благодаря именованному тому. Значение `JWT_SECRET` хранится в локальном `.env` или хранилище секретов. Для смены `API_PORT` согласуй `APP_ORIGIN` с новым адресом приложения.

Пример в `.env.example` предназначен для локальной разработки. При смене пароля обнови `POSTGRES_PASSWORD` и пароль в `DATABASE_URL`. Специальные символы в URL нужно кодировать. В уже инициализированной PostgreSQL смена переменной сама по себе пароль роли не меняет. Для внешней БД используй проверку TLS по требованиям окружения.

Для запуска Go вне Compose задай в окружении `DATABASE_URL` доступной с хоста PostgreSQL, а также `JWT_SECRET`, `JWT_ISSUER`, `JWT_AUDIENCE`, `APP_ORIGIN` и соответствующие настройки cookie. Go читает переменные процесса. `HTTP_ADDR` по умолчанию равен `127.0.0.1:8080`.

```sh
cd backend
go run ./cmd/migrate up
go run ./cmd/api
```

Таймауты HTTP и БД, параметры пула и уровень логов заданы в [шаблоне среды](.env.example). `DB_CHECK_TIMEOUT` должен быть меньше таймаута записи HTTP, а `DB_POOL_MIN_CONNS` не должен превышать `DB_POOL_MAX_CONNS`. При увеличении таймаута graceful shutdown задай больший `API_STOP_GRACE_PERIOD`. Команды проверки миграций и резервного копирования описаны в [документации БД](docs/DATABASE.md).

## Настройка Authentication

| Переменная | Значение |
| --- | --- |
| `JWT_SECRET` | Base64 от 32–128 случайных байт. Для локальной разработки сгенерируй через `openssl rand -base64 32` |
| `JWT_ISSUER`, `JWT_AUDIENCE` | Идентификаторы издателя и аудитории |
| `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL` | 15 минут и 30 дней по умолчанию. Допускаются более короткие сроки |
| `APP_ENV`, `APP_ORIGIN` | В production обязателен точный HTTPS origin. HTTP разрешён в development/test на loopback с явным `COOKIE_SECURE=false` |
| `REFRESH_COOKIE_NAME`, `COOKIE_SECURE` | Имя по умолчанию `focusboard_refresh`, флаг Secure по умолчанию включён. Path, SameSite и HttpOnly заданы сервером |
| `AUTH_REQUEST_TIMEOUT` | 8 секунд, включая операции с БД. Меньше `HTTP_WRITE_TIMEOUT` |
| `AUTH_RATE_WINDOW`, `AUTH_RATE_IP_LIMIT`, `AUTH_RATE_ACCOUNT_LIMIT`, `AUTH_RATE_MAX_KEYS` | 1 минута, 60 запросов на IP, 10 на аккаунт, 8192 ключа |
| `AUTH_PASSWORD_WORKERS` | 2 одновременные операции Argon2id, допустимо 1–8 |

Примеры запросов есть в [документации API](docs/API.md).

## Запуск Frontend

Понадобятся Node.js 26.13.1 и pnpm 11.6.0. Из корня проекта:

```sh
cd frontend
npm install --global pnpm@11.6.0
pnpm install --frozen-lockfile
pnpm dev
```

Открой адрес из вывода Vite. План подключения SPA к API и запуска для ручной приёмки описан в [этапе 5](docs/ROADMAP.md#5-основа-spa-и-экран-авторизации).

## Проверки

Из корня проекта. Для локальной сборки Backend используется Go 1.27.4, как в CI.

```sh
(cd backend && test -z "$(gofmt -l .)" && go mod verify && go vet ./... && go test -race ./... && go build ./...)
(cd frontend && pnpm install --frozen-lockfile && pnpm lint && pnpm format:check && pnpm typecheck && pnpm test && pnpm build)
bash scripts/test-compose.sh
```

Интеграционные тесты Go запускаются при заданном `FOCUSBOARD_TEST_DATABASE_URL` для отдельной базы `focusboard_test`. [GitHub Actions](.github/workflows/ci.yml) поднимает её автоматически. Compose smoke создаёт отдельный тестовый проект, проверяет порядок запуска, потерю и восстановление связи с PostgreSQL и сохранность данных. После прогона тестовые контейнеры останавливаются, тестовый том сохраняется. Compose CI также выполняет полный HTTP-цикл регистрации, входа, обновления сессии и профиля, смены пароля и выхода. Сценарий Tasks проверяет все шесть маршрутов, перестановку и изоляцию. Сценарий Statistics проверяет периоды, часовой пояс, первое Done, soft delete и изоляцию.

```sh
(cd backend && go test ./internal/platform/password -run '^$' -bench '^BenchmarkHash$' -benchtime=8x -benchmem)
python3 scripts/test-auth.py http://127.0.0.1:8080 http://127.0.0.1:8080
python3 scripts/test-tasks.py http://127.0.0.1:8080 http://127.0.0.1:8080
python3 scripts/test-statistics.py http://127.0.0.1:8080 http://127.0.0.1:8080
```

Отдельные HTTP smoke-сценарии создают тестовые аккаунты. Запускай их с локальным API и изолированной тестовой БД.

## Документация

- [Техническое задание](SPECIFICATION.md) и [макеты](docs/mockups/)
- [Инструкции Codex](AGENTS.md) и [дорожная карта](docs/ROADMAP.md)
- [Архитектура](docs/ARCHITECTURE.md) и [решения](docs/DECISIONS.md)
- [База данных](docs/DATABASE.md), [REST API](docs/API.md) и [дизайн-система](docs/DESIGN_SYSTEM.md)
- [История изменений](docs/CHANGELOG.md)
