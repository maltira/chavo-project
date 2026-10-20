# Chavo — backend мессенджера

Микросервисы на Go: личные и групповые чаты, сообщения в реальном времени, статусы «онлайн/офлайн», статусы-баннеры и фигуры за аватаром у профилей и групп, сессии на нескольких устройствах

## Архитектура

| Сервис | Отвечает за |
|---|---|
| `api-gateway` | единственный HTTP API, проверка токенов, WebSocket, доставка событий, «в сети», лимиты |
| `auth-service` | регистрация, вход с OTP по почте, access/refresh-токены, сессии |
| `user-service` | профили, настройки, блокировки |
| `conversation-service` | чаты, группы, сообщения (хранятся зашифрованными), прочтения |

Сервисы общаются только по gRPC, у каждого своя БД PostgreSQL. События идут через Kafka, состояние соединений и сессий хранится в Redis.

## Требования

- Docker с Compose и `make` - для запуска
- Go 1.26 и gcc - для тестов (`-race` требует cgo)
- Node.js 24 и npm - для фронтенда (`services/frontend`)

## Запуск

```bash
cp .env.example .env
# заполнить: JWT_SECRET, POSTGRES_USER / POSTGRES_PASSWORD, *_DB_NAME,
# MESSAGE_ENCRYPTION_KEY (openssl rand -base64 32)

make up      # первый запуск собирает образы - несколько минут
make down    # остановить
```

Миграции применяются автоматически при старте. Генерировать proto не нужно: код в `proto/gen` закоммичен; `make proto` - только после правки `.proto`.


| Адрес | Что там |
|---|---|
| http://localhost:8080 | приложение: REST `/api/...`, WebSocket `/ws` |
| http://localhost:8025 | Mailpit - локальная почта (письма никуда не уходят) |
| http://localhost:8090 | Kafka UI |

Если `make up` падает:
- **порт занят:** 8080, 8025 и 8090 меняются в `.env` (`NGINX_PORT`, `MAILPIT_UI_PORT`, `KAFKA_UI_PORT`); 5432 (PostgreSQL), 6379 (Redis) и 9092 (Kafka) зашиты в compose - остановите то, что их занимает (например, локальный PostgreSQL);
- **нет базы данных:** (`database "…" does not exist`) том PostgreSQL создан до появления этой БД: `make init-dbs`, затем `make up`.

## Фронтенд

`services/frontend` - React + TypeScript + Vite, архитектура FSD, стили на SCSS (CSS Modules и токены в `src/app/styles`).

```bash
make frontend-install   # npm ci
make frontend-dev       # http://localhost:3000, /api и /ws проксируются в nginx (нужен make up)
make frontend-test      # typecheck + eslint + steiger (FSD) + vitest
make frontend-build     # сборка в services/frontend/dist
```

`FRONTEND_URL` в `.env` должен совпадать с адресом dev-сервера (`http://localhost:3000`): по нему Gateway проверяет Origin у WebSocket. Чтобы nginx отдавал сборку вместо заглушки, укажите `FRONTEND_DIST=./services/frontend/dist`.

## Тесты

| Команда | Что запускает |
|---|---|
| `make test` | модульные тесты всех сервисов; ничего поднимать не нужно, тесты с БД пропускаются |
| `make test-integration` | модульные + интеграционные (репозитории против настоящей схемы PostgreSQL) |
| `make e2e` | сквозные: клиент через nginx против поднятого стека |

Один сервис: `make test SERVICE=auth-service` (то же для `test-integration`). Оба `test`-таргета прогоняют все сервисы, даже если какой-то упал, и в конце печатают упавшие.

`make test-integration` сам поднимает временный PostgreSQL (`chavo-test-pg` на порту 15432) и удаляет его после прогона, в том числе при падении. Миграции тесты применяют сами, каждому пакету - своя схема. Другой порт: `TEST_PG_PORT=…`; берите ниже 32768 - выше ядро раздаёт исходящим соединениям, и порт может оказаться занят

Вручную, без make:

```bash
cd services/<сервис> && go test ./...
TEST_DATABASE_URL="postgres://postgres:x@localhost:15432/postgres?sslmode=disable" go test ./...   # нужен PostgreSQL
```

**Сквозные** (`make e2e`) идут против поднятого стека (`make up`), письма читаются из Mailpit. Работает только при почте через Mailpit (в `.env`: `SMTP_HOST=mailpit`, `SMTP_PORT=1025`, `SMTP_USER=noreply@chavo.local`), иначе отказывается стартовать. Перезапускает Gateway. Тест исчерпывает лимит nginx на `/api/auth/` (10 запросов в минуту с IP): первые секунды после прогона вход отвечает 429, полностью лимит восстанавливается примерно за минуту. Локально все запросы приходят в nginx с одного адреса, поэтому это касается и входа в браузере. Тестовые данные удаляет за собой.

## Структура

```text
services/      микросервисы (у каждого свой Go-модуль) и фронтенд (frontend)
proto/         gRPC-контракты и сгенерированный код (make proto)
migrations/    SQL-миграции по сервисам
nginx/         конфигурация nginx
tests/e2e/     сквозные тесты
```
