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

## Authentication и Users

| Метод и путь | Доступ | Успешный ответ |
| --- | --- | --- |
| `POST /api/v1/auth/register` | Точный Origin | 201, access JWT, пользователь и refresh cookie |
| `POST /api/v1/auth/login` | Точный Origin | 200, access JWT, пользователь и refresh cookie |
| `POST /api/v1/auth/refresh` | Точный Origin и refresh cookie | 200, новый access JWT и новая refresh cookie |
| `POST /api/v1/auth/logout` | Точный Origin, refresh cookie при наличии | 204, очистка cookie |
| `GET /api/v1/users/me` | Bearer JWT | 200, публичный профиль |
| `PATCH /api/v1/users/me` | Bearer JWT | 200, обновлённый публичный профиль |
| `PUT /api/v1/users/me/password` | Bearer JWT и точный Origin | 204, отзыв всех refresh-сессий и очистка cookie |

Регистрация принимает `name`, `email`, `password`. Вход принимает `email` и `password`. Имя содержит 1–80 символов после удаления пробелов по краям. Сервер принимает email в ASCII, нормализует регистр и пробелы по краям. Для международных доменов используй Punycode. Максимальная длина адреса после нормализации — 254 символа. Новый пароль содержит 12–128 символов и до 512 байт, используется без обрезки и преобразования.

Успешная аутентификация возвращает `accessToken`, `expiresIn` и `user` с `id`, `name`, `email`, `createdAt`. Access JWT подписан HS256, несёт `sub`, `iss`, `aud`, `exp`, `iat` и `jti`, действует до 15 минут. Все ответы этих маршрутов содержат `Cache-Control: no-store`.

Refresh token приходит только через `Set-Cookie`. Cookie — host-only, HttpOnly, SameSite=Lax, Secure в production, Path=`/api/v1/auth`. Max-Age отсчитывается до абсолютного конца сессии. Для операций с cookie и смены пароля обязателен ровно один `Origin`, совпадающий с `APP_ORIGIN`. Отсутствующий и запрещённый Origin получают 403. CLI должен явно устанавливать заголовок. Cross-origin доступ через CORS не настроен.

Refresh и logout принимают пустое тело. При одновременном refresh одной cookie один запрос получает новую сессию, остальные получают 401 без `Set-Cookie`. Клиенту следует координировать одновременные refresh и завершать их перед logout. При потере ответа после успешной ротации может понадобиться повторный вход. Logout идемпотентен, а ошибки БД возвращаются клиенту. После logout или смены пароля уже выданные stateless JWT действуют до своего `exp`.

Неверный email и неверный пароль при входе одинаково возвращают `401 INVALID_CREDENTIALS`. Дублирующий email получает `409 EMAIL_ALREADY_EXISTS`. Невалидные поля возвращают 422 с `error.fields`, некорректный JSON и тела больше 64 КиБ — 400. При достижении лимита ответ содержит `429 RATE_LIMITED` и `Retry-After` в секундах.

### Локальные запросы

Команды рассчитаны на тестовую БД, запущенную по README. Нужен `jq`.

```sh
base=http://127.0.0.1:8080/api/v1
origin=http://127.0.0.1:8080
jar=$(mktemp)
trap 'rm -f "$jar"' EXIT

access=$(curl -fsS -c "$jar" -H "Origin: $origin" -H 'Content-Type: application/json' \
  -d '{"name":"Example","email":"alex@example.com","password":"example-local-passphrase"}' \
  "$base/auth/register" | jq -r .accessToken)
curl -fsS -H "Authorization: Bearer $access" "$base/users/me"
curl -fsS -b "$jar" -c "$jar" -X POST -H "Origin: $origin" "$base/auth/refresh" | jq '.user'
curl -fsS -b "$jar" -c "$jar" -X POST -H "Origin: $origin" "$base/auth/logout"
```

Для полного цикла используется [HTTP smoke](../scripts/test-auth.py).

## Маршруты по этапам

| Пути под `/api/v1` | Ветка реализации | Доступ |
| --- | --- | --- |
| `GET /health`, `GET /ready` | Реализовано на этапе 1 | Public liveness, internal readiness |
| `POST /auth/{register,login,refresh,logout}` | Реализовано на этапе 2 | Public или refresh cookie по операции |
| `GET /users/me`, `PATCH /users/me`, `PUT /users/me/password` | Реализовано на этапе 2 | Bearer JWT |
| `GET /tasks`, `POST /tasks`, `GET/PATCH/DELETE /tasks/{id}`, `PATCH /tasks/{id}/move` | `feat/task-management` | Bearer JWT |
| `GET /statistics` | `feat/statistics-api` | Bearer JWT |

JSON использует camelCase, timestamps — RFC 3339, дедлайн — строку YYYY-MM-DD. Ошибки возвращают объект `error` со стабильным `code`, коротким `message` и при необходимости `fields`. Ответ 204 не содержит body. API возвращает 404 для отсутствующей или чужой задачи.
