# FocusBoard

Персональное приложение для работы с задачами. В MVP входят личная Kanban-доска, сохранение порядка, профиль и статистика по первому закрытию каждой задачи.

Текущее состояние — этап 0. Go/Chi и React/Vite имеют собственные точки входа. Клиент показывает заголовок приложения, сервер подготавливает HTTP lifecycle. Прикладные маршруты, PostgreSQL и Compose относятся к следующим этапам. Порядок работ описан в [дорожной карте](docs/ROADMAP.md).

## Локальный запуск каркаса

Понадобятся Go 1.27.4 и Node.js 26.13.1, как в CI, а также pnpm 11.6.0. Версии зависимостей находятся в Go modules и pnpm lockfile.

Backend из корня проекта:

```sh
cd backend
go run ./cmd/api
```

API слушает `127.0.0.1:8080`. Переопредели адрес переменной `HTTP_ADDR`. В текущем каркасе запросы получают 404 до реализации endpoints.

Frontend в отдельном терминале из корня:

```sh
cd frontend
npm install --global pnpm@11.6.0
pnpm install --frozen-lockfile
pnpm dev
```

Открой адрес из вывода Vite. `.env.example` описывает настройки будущей среды и содержит значения для локальной разработки. Для запуска текущих каркасов файл `.env` не требуется.

## Проверки

Из корня проекта:

```sh
(cd backend && gofmt -l cmd && go mod verify && go vet ./... && go test ./... && go build ./cmd/api)
(cd frontend && pnpm lint && pnpm format:check && pnpm typecheck && pnpm test && pnpm build)
```

`gofmt -l cmd` должен вывести пустой список. Эти проверки входят в [GitHub Actions](.github/workflows/ci.yml). Для форматирования файлов фронтенда используй `pnpm format` из `frontend`. PostgreSQL, Goose, Compose и Playwright подключаются по мере появления проверяемых сценариев.

## Документация

- [Техническое задание](SPECIFICATION.md) и [макеты](docs/mockups/)
- [Инструкции Codex](AGENTS.md) и [дорожная карта](docs/ROADMAP.md)
- [Архитектура](docs/ARCHITECTURE.md) и [решения](docs/DECISIONS.md)
- [База данных](docs/DATABASE.md), [REST API](docs/API.md) и [дизайн-система](docs/DESIGN_SYSTEM.md)
- [История изменений](docs/CHANGELOG.md)
