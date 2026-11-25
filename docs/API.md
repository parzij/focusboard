# REST API

На этапе 0 Chi router запускается без зарегистрированных endpoints. Планируемые пути, модели, примеры JSON и коды ошибок описаны в [§8 ТЗ](../SPECIFICATION.md#8-rest-api). OpenAPI появится вместе с первыми реализованными операциями на этапе 1 и станет машиночитаемым контрактом.

| Пути под `/api/v1` | Ветка реализации | Доступ |
| --- | --- | --- |
| `GET /health`, `GET /ready` | `feat/database-foundation` | Public liveness, internal readiness |
| `POST /auth/{register,login,refresh,logout}` | `feat/authentication` | Public или refresh cookie по операции |
| `GET /users/me`, `PATCH /users/me`, `PUT /users/me/password` | `feat/authentication` | Bearer JWT |
| `GET /tasks`, `POST /tasks`, `GET/PATCH/DELETE /tasks/{id}`, `PATCH /tasks/{id}/move` | `feat/task-management` | Bearer JWT |
| `GET /statistics` | `feat/statistics-api` | Bearer JWT |

JSON использует camelCase, timestamps — RFC 3339, дедлайн — строку YYYY-MM-DD. Ошибки возвращают объект `error` со стабильным `code`, коротким `message` и при необходимости `fields`. Ответ 204 не содержит body. API возвращает 404 для отсутствующей или чужой задачи.
