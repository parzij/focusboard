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

## Задачи

Все операции под `/api/v1` требуют `Authorization: Bearer <access-token>`. UUID владельца берётся из JWT. Первые две операции работают с доской текущего пользователя. Операции с `{id}` находят только активную задачу этого пользователя.

| Метод и путь | Успех | Результат |
| --- | --- | --- |
| `GET /tasks` | 200 | `{"tasks":[]}` для пустой доски. Остальные доски отсортированы по статусу и позиции |
| `POST /tasks` | 201 | Созданный Task в конце выбранной колонки |
| `GET /tasks/{id}` | 200 | Task |
| `PATCH /tasks/{id}` | 200 | Обновлённый Task. При смене статуса карточка добавляется в конец новой колонки |
| `PATCH /tasks/{id}/move` | 200 | `{"tasks":[...]}` с каноническим снимком зафиксированной доски |
| `DELETE /tasks/{id}` | 204 | Пустое тело. Задача скрывается, история сохраняется |

Публичный Task содержит следующие поля.

```json
{
  "id": "fc02661e-ee75-4494-a5c7-06a67d272521",
  "title": "Prepare a release",
  "description": "",
  "status": "not_started",
  "position": 0,
  "dueDate": null,
  "createdAt": "2026-11-25T09:00:00Z",
  "updatedAt": "2026-11-25T09:00:00Z"
}
```

Порядок колонок фиксирован. `not_started` (Not started), `in_progress` (In progress), `finish` (Finish), `done` (Done). Допустимы любые переходы. `createdAt` и `updatedAt` передаются в RFC 3339 UTC.

- `title` обязателен для создания и содержит 1–120 символов Unicode после удаления пробелов по краям. `description` содержит до 2000 символов Unicode, по умолчанию пустая строка. Нулевой символ в тексте отклоняется.
- Статус при создании по умолчанию `not_started`. `dueDate` по умолчанию `null` и принимает календарную дату в формате `YYYY-MM-DD` с годом от 0001 до 9999. Прошедшие даты допустимы.
- Обычный PATCH принимает только `title`, `description`, `status`, `dueDate`. Пропущенные поля сохраняются, `dueDate: null` очищает дату. Для остальных полей `null` запрещён, пустой PATCH отклоняется.
- Для move оба поля обязательны. `position` — целый неотрицательный индекс вставки после логического удаления перемещаемой карточки, максимум равен длине получившейся целевой колонки. Например, перенос A на индекс 2 в колонке A, B, C даёт B, C, A.
- Для всех JSON-запросов действует лимит 64 KiB. Неизвестные и повторяющиеся поля, неверные типы и несколько объектов в одном теле отклоняются.

Все изменения одной доски сериализуются. Вставка рассчитывается по актуальному порядку в момент выполнения операции. Допустимый индекс с устаревшего снимка применяется к текущему состоянию, поскольку в контракте отсутствует версия доски. Конкурентные PATCH разных полей сохраняют оба изменения. Для одного поля действует значение последней зафиксированной операции. Изменения индекса уплотняют затронутые колонки, сохраняя относительный порядок остальных задач.

| HTTP | Код | Условие |
| --- | --- | --- |
| 400 | `VALIDATION_ERROR` | Некорректное тело JSON |
| 401 | `UNAUTHORIZED` | Отсутствующий, недействительный или просроченный JWT |
| 404 | `TASK_NOT_FOUND` | Задача отсутствует, удалена или принадлежит другому аккаунту |
| 409 | `ORDER_CONFLICT` | Обнаружен разрыв в проверяемой колонке или нарушено ограничение уникальности позиций, в том числе при commit |
| 422 | `VALIDATION_ERROR` | Некорректный UUID или значение поля, в том числе индекс за пределами колонки |
| 500 | `INTERNAL_ERROR` | Внутренняя ошибка |
| 503 | `SERVICE_UNAVAILABLE` | Истёк таймаут операции |

Локальные запросы с `access` из ответа регистрации или входа и `id` из ответа создания.

```sh
base=http://127.0.0.1:8080/api/v1
curl -fsS -H "Authorization: Bearer $access" "$base/tasks"
curl -fsS -H "Authorization: Bearer $access" -H 'Content-Type: application/json' \
  -d '{"title":"Prepare a release","dueDate":"2026-11-30"}' "$base/tasks"
curl -fsS -H "Authorization: Bearer $access" "$base/tasks/$id"
curl -fsS -X PATCH -H "Authorization: Bearer $access" -H 'Content-Type: application/json' \
  -d '{"dueDate":null}' "$base/tasks/$id"
curl -fsS -X PATCH -H "Authorization: Bearer $access" -H 'Content-Type: application/json' \
  -d '{"status":"done","position":0}' "$base/tasks/$id/move"
curl -fsS -X DELETE -H "Authorization: Bearer $access" "$base/tasks/$id"
```

Полный [HTTP smoke задач](../scripts/test-tasks.py) создаёт два тестовых аккаунта и проверяет операции доски и изоляцию.

## Статистика

`GET /api/v1/statistics` требует `Authorization: Bearer <access-token>`. Для каждой задачи владельца считается только первое событие Done за всю историю. Учитываются задачи, созданные сразу в Done, и закрытия задач, которые впоследствии переоткрыли или удалили. Finish и повторное Done не увеличивают счётчики.

| Параметр | Правила |
| --- | --- |
| `period` | `days` по умолчанию. `days` возвращает семь дат текущей недели с понедельника по воскресенье. `weeks` возвращает восемь недель, включая текущую, с датой понедельника в каждом интервале |
| `timezone` | `UTC` по умолчанию или имя зоны IANA, например `Europe/Moscow`, `America/New_York` либо `Asia/Kolkata`. Для знака `+` используй URL-кодирование `%2B` |

Явно пустые, повторяющиеся и неизвестные параметры, включая `userId`, отклоняются. Произвольные пути, `Local` и системные псевдонимы отклоняются. Даты и интервалы определяются в выбранной зоне, включая переходы DST. Подсчёт использует полуоткрытые границы и исключает события позже момента отчёта.

Ответ содержит `period`, `timezone`, хронологический массив `buckets` с полями `startDate` в формате `YYYY-MM-DD` и целым `count`, а также `thisWeek` и `thisMonth`. Пустые интервалы и будущие дни заполнены нулями. Сводные числа относятся к текущим календарным неделе и месяцу независимо от `period`. Примеры обоих ответов приведены в [OpenAPI](openapi.yaml).

| HTTP | Код | Условие |
| --- | --- | --- |
| 200 | | Отчёт. Заголовок `Cache-Control: no-store` |
| 401 | `UNAUTHORIZED` | Отсутствующий, недействительный или просроченный JWT |
| 422 | `VALIDATION_ERROR` | Ошибка параметров с деталями в `error.fields` |
| 500 | `INTERNAL_ERROR` | Внутренняя ошибка |
| 503 | `SERVICE_UNAVAILABLE` | Истёк таймаут запроса |

```sh
base=http://127.0.0.1:8080/api/v1
curl -fsS -H "Authorization: Bearer $access" "$base/statistics"
curl -fsS -G -H "Authorization: Bearer $access" \
  --data-urlencode 'period=weeks' --data-urlencode 'timezone=Europe/Moscow' \
  "$base/statistics"
```
