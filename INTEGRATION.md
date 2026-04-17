# Интеграция бэкенда мероприятий в LASTOP

## Что добавлено

| Файл | Куда в проекте | Действие |
|------|---------------|----------|
| `models/events.go` | `models/events.go` | скопировать |
| `handlers/events.go` | `handlers/events.go` | скопировать |
| `database/events_schema.sql` | выполнить в БД | применить |
| `database/events_seed.sql` | выполнить в БД | применить (опционально) |
| `main_events_routes.go` | `main.go` | вставить маршруты |

---

## Шаг 1: Модели

Скопируйте `models/events.go` в папку `models/` проекта.

> Если в `models/models.go` уже есть `StringSlice` или похожий тип —
> переименуйте или удалите дублирование. Тип `Speaker` уникален.

---

## Шаг 2: БД — применить схему

```bash
# Вариант А: через docker compose (локально)
docker compose exec postgres psql -U postgres -d lastop -f /dev/stdin < database/events_schema.sql

# Вариант Б: напрямую к локальному PostgreSQL
psql "$DATABASE_URL" -f database/events_schema.sql

# Вариант В: Railway / удалённый хост
# Скопируйте содержимое events_schema.sql и выполните в Query Editor Railway
```

Потом (опционально) добавьте тестовые данные:
```bash
psql "$DATABASE_URL" -f database/events_seed.sql
```

---

## Шаг 3: Хендлер

Скопируйте `handlers/events.go` в `handlers/events.go` проекта.

Функции в файле используют:
- `database.DB` — переменная `*sql.DB` из вашего `database/db.go`
- `r.Context().Value("userID")` — устанавливается middleware авторизации

---

## Шаг 4: Маршруты в main.go

Найдите в `main.go` блок регистрации защищённых маршрутов.
Обычно он выглядит так:

```go
// Уже есть в проекте:
auth.HandleFunc("/resumes",     handlers.ResumesList).Methods("GET")
auth.HandleFunc("/jobs",        handlers.JobsList).Methods("GET")
// ...

// Добавьте после них:
auth.HandleFunc("/events",               handlers.EventsList).Methods("GET")
auth.HandleFunc("/events",               handlers.EventCreate).Methods("POST")
auth.HandleFunc("/events/stats",         handlers.EventsStatsHandler).Methods("GET")
auth.HandleFunc("/events/my",            handlers.EventsMy).Methods("GET")
auth.HandleFunc("/events/{id}",          handlers.EventGet).Methods("GET")
auth.HandleFunc("/events/{id}/register", handlers.EventRegister).Methods("POST")
auth.HandleFunc("/events/{id}/register", handlers.EventUnregister).Methods("DELETE")
auth.HandleFunc("/events/{id}/view",     handlers.EventView).Methods("POST")
```

> **Важно:** маршруты `/events/stats` и `/events/my` должны быть
> зарегистрированы **до** `/events/{id}`, иначе gorilla/mux перехватит
> "stats" и "my" как id.

---

## Шаг 5: Фронтенд (events.html)

Скопируйте `web/events.html` (уже создан) в папку `web/` проекта.
Сервер раздаёт `web/` как статику, поэтому маршрут `/events.html`
заработает автоматически.

Если у вас в `main.go` есть явная регистрация HTML-страниц,
добавьте маршрут `/events` или `/events.html`:

```go
// Для gorilla/mux:
r.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "web/events.html")
})
r.HandleFunc("/events.html", func(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "web/events.html")
})
```

---

## API Reference

### GET /api/events

Возвращает список мероприятий. Параметры запроса:

| Параметр | Тип    | Описание |
|---------|--------|---------|
| `date`  | string | YYYY-MM-DD — конкретный день |
| `type`  | string | webinar \| workshop \| conference \| networking \| roundtable |
| `format`| string | online \| offline \| hybrid |
| `q`     | string | полнотекстовый поиск по title, description, category |
| `limit` | int    | макс. кол-во (по умолч. 200) |
| `offset`| int    | смещение |

Ответ:
```json
{
  "events": [ ...Event ],
  "total": 10
}
```

### GET /api/events/stats

Ответ:
```json
{
  "stats": {
    "total": 318,
    "today": 3,
    "upcoming": 124,
    "categories": [
      { "type": "webinar", "label": "Вебинар", "count": 148 }
    ]
  }
}
```

### GET /api/events/my

Мероприятия текущего пользователя (на которые записан), дата ≥ сегодня.

### GET /api/events/{id}

Одно мероприятие + инкремент views_count (в фоне).

### POST /api/events

Создать мероприятие. Тело:
```json
{
  "title": "Вебинар по таможне",
  "type": "webinar",
  "format": "online",
  "category": "ВЭД и таможня",
  "city": "",
  "date": "2026-04-25",
  "time_start": "10:00",
  "time_end": "12:00",
  "fee": 0,
  "seats_total": 0,
  "description": "...",
  "cover": "base64...",
  "tags": ["таможня"],
  "speakers": [{"name": "Иванов", "role": "Эксперт"}]
}
```

Ответ: `{ "id": "uuid" }` со статусом 201.

### POST /api/events/{id}/register

Записаться на мероприятие. Возвращает 409, если мест нет.

### DELETE /api/events/{id}/register

Отменить регистрацию.

### POST /api/events/{id}/view

Инкремент счётчика просмотров (возвращает 204).

---

## Структура Event (JSON)

```json
{
  "id": "uuid",
  "title": "string",
  "type": "webinar",
  "format": "online",
  "category": "string",
  "city": "string (optional)",
  "date": "2026-04-17",
  "time_start": "10:00",
  "time_end": "11:30",
  "duration_min": 90,
  "fee": 0,
  "seats_total": 0,
  "description": "string",
  "cover": "string (base64 or URL, optional)",
  "organizer_id": "uuid",
  "organizer_name": "string",
  "tags": ["string"],
  "speakers": [{"name": "string", "role": "string"}],
  "registered_count": 42,
  "views_count": 100,
  "comments_count": 5,
  "is_registered": false,
  "created_at": "2026-04-17T10:00:00Z"
}
```

---

## Триггер пересчёта мест

В `events_schema.sql` создан PostgreSQL-триггер:
```sql
AFTER INSERT OR DELETE ON event_registrations
FOR EACH ROW EXECUTE FUNCTION update_event_registered_count();
```

Он автоматически поддерживает `events.registered_count` в актуальном состоянии.
Нет необходимости вручную инкрементировать/декрементировать счётчик в Go-коде.

---

## Проверка после деплоя

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/events/stats
# {"stats":{"total":10,"today":3,"upcoming":7,...}}

curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/events?date=2026-04-17"
# {"events":[...],"total":3}
```
