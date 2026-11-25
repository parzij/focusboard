# REST API

Реализованные операции описаны в [OpenAPI](openapi.yaml). Планируемые модели и коды ошибок описаны в [§8 ТЗ](../SPECIFICATION.md#8-rest-api).

## Пробы состояния

| Запрос | HTTP | Ответ |
| --- | --- | --- |
| `GET /api/v1/health` | 200 | `{"status":"ok"}` |
| `GET /api/v1/ready` | 200 | `{"status":"ready"}` |
| `GET /api/v1/ready` | 503 | `{"error":{"code":"SERVICE_UNAVAILABLE","message":"Service temporarily unavailable."}}` |

Health проверяет HTTP-сервер. Readiness выполняет ограниченный по времени PostgreSQL `Ping`. Ответы имеют `Content-Type: application/json`, `Cache-Control: no-store` и `X-Request-ID`.

На этапе 9 Caddy должен закрывать `/api/v1/ready`, `/api/v1/docs` и дочерние пути документации до обработки общего прокси `/api/*`. При проксировании сохраняется префикс `/api/v1`. Liveness доступен через публичный прокси.

## Маршруты по этапам

| Пути под `/api/v1` | Ветка реализации | Доступ |
| --- | --- | --- |
| `GET /health`, `GET /ready` | Реализовано на этапе 1 | Public liveness, internal readiness |
| `POST /auth/{register,login,refresh,logout}` | `feat/authentication` | Public или refresh cookie по операции |
| `GET /users/me`, `PATCH /users/me`, `PUT /users/me/password` | `feat/authentication` | Bearer JWT |
| `GET /tasks`, `POST /tasks`, `GET/PATCH/DELETE /tasks/{id}`, `PATCH /tasks/{id}/move` | `feat/task-management` | Bearer JWT |
| `GET /statistics` | `feat/statistics-api` | Bearer JWT |

JSON использует camelCase, timestamps — RFC 3339, дедлайн — строку YYYY-MM-DD. Ошибки возвращают объект `error` со стабильным `code`, коротким `message` и при необходимости `fields`. Ответ 204 не содержит body. API возвращает 404 для отсутствующей или чужой задачи.
