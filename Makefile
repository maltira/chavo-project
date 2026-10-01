include .env
export

# ──────────────────────────────────────────────
# Переменные
# ──────────────────────────────────────────────

COMPOSE          := docker compose
MIGRATE_IMG      := migrate/migrate:v4.19.1
NETWORK          := chavo-network

DB_AUTH_URL      := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(AUTH_DB_NAME)?sslmode=disable
DB_USER_URL      := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(USER_DB_NAME)?sslmode=disable
DB_MESSAGE_URL   := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(MESSAGE_DB_NAME)?sslmode=disable

.PHONY: up down restart \
        init-dbs \
        migrate-up migrate-down \
        migrate-auth-down migrate-user-down migrate-message-down \
        psql db-tables clean-db clean-redis clean-kafka clean-data \
        kafka-topics kafka-ui


# ──────────────────────────────────────────────
# Инфраструктура
# ──────────────────────────────────────────────

up:
	@$(COMPOSE) up -d --build

down:
	@$(COMPOSE) down

restart: down up

# ──────────────────────────────────────────────
# База данных
# ──────────────────────────────────────────────

init-dbs:
	@$(COMPOSE) up -d --wait postgres
	@echo "Инициализация баз данных..."
	@docker exec chavo-postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "\
		SELECT 'CREATE DATABASE $(AUTH_DB_NAME)' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$(AUTH_DB_NAME)')\gexec; \
		SELECT 'CREATE DATABASE $(USER_DB_NAME)' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$(USER_DB_NAME)')\gexec; \
		SELECT 'CREATE DATABASE $(MESSAGE_DB_NAME)' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$(MESSAGE_DB_NAME)')\gexec;"
	@echo "Базы данных готовы."

psql:
	@docker exec -it chavo-postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

db-tables:
	@echo "=== Auth DB ($(AUTH_DB_NAME)) ==="
	@docker exec chavo-postgres psql -U $(POSTGRES_USER) -d $(AUTH_DB_NAME) -c "\dt" 2>/dev/null || true
	@echo "=== User DB ($(USER_DB_NAME)) ==="
	@docker exec chavo-postgres psql -U $(POSTGRES_USER) -d $(USER_DB_NAME) -c "\dt" 2>/dev/null || true
	@echo "=== Message DB ($(MESSAGE_DB_NAME)) ==="
	@docker exec chavo-postgres psql -U $(POSTGRES_USER) -d $(MESSAGE_DB_NAME) -c "\dt" 2>/dev/null || true

clean-db:
	@$(COMPOSE) up -d --wait postgres
	@echo "Очистка таблиц PostgreSQL..."
	@docker exec chavo-postgres psql -U $(POSTGRES_USER) -d $(AUTH_DB_NAME) -c "\
		TRUNCATE TABLE users, email_verifications, refresh_tokens CASCADE;" 2>/dev/null || true
	@docker exec chavo-postgres psql -U $(POSTGRES_USER) -d $(USER_DB_NAME) -c "\
		TRUNCATE TABLE profiles, user_settings, user_blocks CASCADE;" 2>/dev/null || true
	@docker exec chavo-postgres psql -U $(POSTGRES_USER) -d $(MESSAGE_DB_NAME) -c "\
		TRUNCATE TABLE conversations, conversation_members, conversation_join_requests, messages, message_receipts CASCADE;" 2>/dev/null || true
	@echo "Таблицы БД успешно очищены."

# ──────────────────────────────────────────────
# Redis
# ──────────────────────────────────────────────

clean-redis:
	@$(COMPOSE) up -d --wait redis
	@echo "Очистка данных Redis..."
	@docker exec chavo-redis redis-cli FLUSHALL
	@echo "Redis успешно очищен."

# ──────────────────────────────────────────────
# Миграции
# ──────────────────────────────────────────────

migrate-up:
	@$(COMPOSE) up migrate --force-recreate

migrate-auth-down:
	@docker run --rm --network $(NETWORK) \
		-v $(shell pwd)/migrations/auth:/migrations \
		$(MIGRATE_IMG) \
		-path=/migrations \
		-database="$(DB_AUTH_URL)" \
		down 1

migrate-user-down:
	@docker run --rm --network $(NETWORK) \
		-v $(shell pwd)/migrations/user:/migrations \
		$(MIGRATE_IMG) \
		-path=/migrations \
		-database="$(DB_USER_URL)" \
		down 1

migrate-message-down:
	@docker run --rm --network $(NETWORK) \
		-v $(shell pwd)/migrations/message:/migrations \
		$(MIGRATE_IMG) \
		-path=/migrations \
		-database="$(DB_MESSAGE_URL)" \
		down 1

migrate-down: migrate-auth-down migrate-user-down migrate-message-down


# ──────────────────────────────────────────────
# Kafka
# ──────────────────────────────────────────────

clean-kafka:
	@$(COMPOSE) up -d --wait kafka
	@echo "Очистка топиков Kafka..."
	@docker exec chavo-kafka /bin/sh -c '\
		kafka-topics --delete --if-exists --bootstrap-server localhost:9092 --topic auth-events; \
		kafka-topics --delete --if-exists --bootstrap-server localhost:9092 --topic user-events; \
		kafka-topics --delete --if-exists --bootstrap-server localhost:9092 --topic conversation-events; \
		kafka-topics --delete --if-exists --bootstrap-server localhost:9092 --topic message-events;'
	@$(COMPOSE) up kafka-init --force-recreate
	@echo "Топики Kafka успешно пересозданы."

kafka-topics:
	@docker exec chavo-kafka kafka-topics \
		--list --bootstrap-server localhost:9092

kafka-ui:
	@xdg-open http://localhost:$(KAFKA_UI_PORT) 2>/dev/null || \
		echo "Открой в браузере: http://localhost:$(KAFKA_UI_PORT)"

clean-data: clean-db clean-kafka clean-redis