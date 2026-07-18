## 1. Обзор проекта

**Chavo** — бэкенд для мессенджера, построенный по микросервисной архитектуре.
Монорепозиторий с общим `docker-compose.yaml` и Makefile на корневом уровне.

| Сервис | Статус | Порт | БД |
|---|---|---|---|
| `auth-service` | ✅ Реализован | 8001 | PostgreSQL (`AUTH_DB_NAME`) |
| `user-service` | 🔧 В разработке | 8002 | PostgreSQL (`USER_DB_NAME`) |
| `chat-service` | 🔧 Запланирован | — | PostgreSQL (`CHAT_DB_NAME`) |

**Стек:**
- Go 1.26, Gin, pgx/v5, redis/go-redis/v9, zap, golang-jwt/v5, bcrypt
- PostgreSQL 16, Redis 7, Docker Compose
- Миграции: `migrate/migrate` v4.19.1

## 2. Структура репозитория

```
chavo-project-backend/
├── Makefile                     # Команды для dev-окружения и миграций
├── docker-compose.yaml          # Инфраструктура (postgres, redis, port-forwarder, сервисы)
├── .env / .env.example          # Переменные окружения
├── migrations/
│   ├── auth/                    # SQL-миграции auth-сервиса
│   ├── user/
│   └── chat/
└── services/
    └── auth-service/
        ├── Dockerfile           # Multi-stage build (golang:1.26-alpine → alpine:3.22)
        ├── go.mod
        ├── cmd/main.go          # Точка входа: wire-up зависимостей + HTTP сервер
        ├── config/config.go     # Загрузка конфига из env-переменных
        ├── pkg/
        │   ├── db/              # pgxpool init + close
        │   └── redis/           # redis.Client init + close
        └── internal/
            ├── apperror/        # Sentinel errors + HTTP-маппинг + user messages
            ├── dto/             # Request/Response структуры (gin binding)
            ├── email/           # EmailSender interface + SMTP impl + HTML templates
            ├── handler/http/    # Gin-хендлеры (auth, otp, token)
            ├── logger/          # zap-логгер (dev/prod mode)
            ├── models/          # Доменные модели (User, RefreshToken, OTPCode)
            ├── repository/      # pgx-реализации репозиториев
            ├── router/          # Gin router setup
            ├── service/         # Бизнес-логика (auth, otp, token)
            └── utils/           # JWT, токены, OTP, cookie, device, events
```

---

## 3. Команды разработки (Makefile)

### Инфраструктура
```bash
make ps-up              # Запустить PostgreSQL
make ps-down            # Остановить PostgreSQL
make ps-cleanup         # Полное удаление PostgreSQL + данных (с подтверждением)
make env-port-forward   # Пробросить PostgreSQL на localhost:5432
make env-port-close     # Закрыть проброс портов
```

### Запуск сервисов
```bash
make chavo-auth-run     # Собрать и запустить auth-service в Docker
make chavo-auth-down    # Остановить auth-service
```

### Миграции
```bash
# Применить миграции
make migrate-auth-up
make migrate-user-up
make migrate-chat-up

# Откатить последнюю миграцию
make migrate-auth-down
make migrate-user-down
make migrate-chat-down

# Создать новый файл миграции
make migrate-create-auth seq=<name>    # Пример: make migrate-create-auth seq=add_sessions
make migrate-create-user seq=<name>
make migrate-create-chat seq=<name>
```

## 4. Архитектура auth-service

### Слои (снизу вверх)
```
DB/Redis
  └── repository/    (pgx queries, интерфейсы)
        └── service/ (бизнес-логика, оркестрация)
              └── handler/ (HTTP, gin binding, cookies)
                    └── router/ (маршруты)
```

### Ключевые соглашения кода

1. **Ошибки**: только sentinel errors из `apperror`. Новый error → добавить в `errors.go`,
   `HTTPCode()` и `UserMessage()`. Никогда не прокидывать строки баз данных наружу.

2. **Репозитории**: интерфейс + приватная структура. `scanUser` / `scanToken` — DRY-хелперы.
   `isDuplicateKey()` — проверка PG error code `23505`.

3. **Email**: шаблоны встроены через `embed.FS` (компилируются в бинарь). Отправка всегда
   в горутине с `defer recover()` — email-сбой не возвращается вызывающей стороне.

4. **Redis key conventions**:
   - `verify:<token>` → `user_id` (TTL 15m)
   - `verify:ref:<user_id>` → `token` (TTL 15m, для инвалидации при re-register)
   - `reset:password:<hash>` → `user_id`
   - `reset:password:ref:<user_id>` → `hash`
   - `blacklist:jti:<jti>` → `"1"` (TTL = AccessTokenDuration)

5. **Аутентификация flow**:
   - Login: email+pass → OTP на почту → `POST /auth/login/verify` → access+refresh tokens
   - Refresh token: httpOnly cookie, ротация при каждом refresh
   - Access token: JWT (HS256), short-lived, JTI blacklist в Redis при logout/refresh

6. **Межсервисное взаимодействие**: синхронный HTTP с `X-Internal-Secret` заголовком.
   При верификации аккаунта → POST на user-service для создания профиля (3 retry, linear backoff).

7. **Soft delete**: email мутируется в `<email>_deleted_<id[:8]>` чтобы освободить слот
   в `UNIQUE CONSTRAINT users_email_unique`.

8. **Cookie TTL**: `SetAuthCookies` принимает `maxAge int` (секунды). Хендлеры вычисляют
   значение через `int(cfg.RefreshTokenDuration.Seconds())` — никаких магических чисел.

9. **ON CONFLICT**: `users` таблица использует `CONSTRAINT users_email_unique UNIQUE (email)`
   (полный, не partial) чтобы `ON CONFLICT (email)` в `Create()` работал корректно.
   Для запросов по активным пользователям есть отдельный `idx_users_email_active WHERE deleted_at IS NULL`.

## 5. API Routes

```
POST   /auth/register              Регистрация (upsert: повтор если не верифицирован)
PUT    /auth/register/verify       Подтверждение email по ссылке (?token=...)
POST   /auth/login                 Вход → отправляет OTP
PUT    /auth/login/verify          Подтверждение OTP → выдаёт access+refresh tokens
POST   /auth/refresh               Ротация refresh token (из cookie)
POST   /auth/logout                Выход с текущего устройства
POST   /auth/logout/all            Выход со всех устройств
POST   /auth/logout/:token_id      Завершить конкретную сессию
POST   /auth/forgot-password       Сброс пароля → письмо со ссылкой
POST   /auth/reset-password        Применить новый пароль по токену
GET    /auth/me                    Данные текущего пользователя
POST   /auth/change/pass           Инициировать смену пароля → OTP
PUT    /auth/change/pass/verify    Подтвердить смену пароля
POST   /auth/change/email          Инициировать смену email → OTP
PUT    /auth/change/email/verify   Подтвердить смену email
POST   /auth/account/delete        Инициировать удаление аккаунта → OTP
POST   /auth/account/delete/verify Подтвердить удаление аккаунта
GET    /auth/sessions              Список активных сессий
```