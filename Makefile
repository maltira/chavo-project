include .env
export

export PROJECT_ROOT=$(shell pwd)

# make env-up - Запуск PostgreSQL
env-up:
	@docker compose up -d chavo-postgres

# make env-down - Остановка PostgreSQL
env-down:
	@docker compose down chavo-postgres

# make env-cleanup - ПОЛНОЕ удаление окружения (с потерей данных!)
env-cleanup:
	@read -p "Очистить все volume файлы окружения? Опасность утери данных. [y/N]: " ans; \
	if [ "$$ans" = "y" ]; then \
	  docker compose down chavo-postgres && \
	  sudo rm -rf out/pgdata && \
	  echo "Файлы окружения очищены"; \
	else \
	  echo "Очистка окружения отменена"; \
	fi

# make env-port-forward - Проброс портов на хост
env-port-forward:
	@docker compose up -d port-forwarder

# make env-port-close - Закрыть проброс портов
env-port-close:
	@docker compose down port-forwarder

# Миграции - это SQL файлы в папке /migrations, которые последовательно
# изменяют схему базы данных. Каждая миграция имеет:
#   - up.sql   - применяет изменения (движение вперёд)
#   - down.sql - откатывает изменения (движение назад)

MIGRATE_AUTH_PATH := /migrations/auth
MIGRATE_USER_PATH := /migrations/user
MIGRATE_CHAT_PATH := /migrations/chat

# make migrate-create - Создает новый файл миграции
migrate-create-auth:
	@if [ -z "$(seq)" ]; then \
		echo "Отсутствует параметр seq. Пример: make migrate-make-auth seq=init"; \
		exit 1; \
	fi; \
	docker compose run --rm chavo-postgres-migrate \
		create -ext sql -dir $(MIGRATE_AUTH_PATH) -seq "$(seq)"
migrate-create-user:
	@if [ -z "$(seq)" ]; then \
		echo "Отсутствует параметр seq. Пример: make migrate-make-user seq=init"; \
		exit 1; \
	fi; \
	docker compose run --rm chavo-postgres-migrate \
		create -ext sql -dir $(MIGRATE_USER_PATH) -seq "$(seq)"
migrate-create-chat:
	@if [ -z "$(seq)" ]; then \
		echo "Отсутствует параметр seq. Пример: make migrate-make-chat seq=init"; \
		exit 1; \
	fi; \
	docker compose run --rm chavo-postgres-migrate \
		create -ext sql -dir $(MIGRATE_CHAT_PATH) -seq "$(seq)"


# make migrate-up - Применить все ожидающие миграции
migrate-auth-up:
	@make migrate-action db=${AUTH_DB_NAME} path=$(MIGRATE_AUTH_PATH) action=up
migrate-user-up:
	@make migrate-action db=${USER_DB_NAME}  path=$(MIGRATE_USER_PATH) action=up
migrate-chat-up:
	@make migrate-action db=${CHAT_DB_NAME}  path=$(MIGRATE_CHAT_PATH) action=up

# make migrate-down - Откатить последнюю миграцию
migrate-auth-down:
	@make migrate-action db=${AUTH_DB_NAME} path=$(MIGRATE_AUTH_PATH) action=down
migrate-user-down:
	@make migrate-action db=${USER_DB_NAME} path=$(MIGRATE_USER_PATH) action=down
migrate-chat-down:
	@make migrate-action db=${CHAT_DB_NAME} path=$(MIGRATE_CHAT_PATH) action=down

# migrate-action - Внутренняя команда для выполнения миграций (не для прямого вызова)
migrate-action:
	@if [ -z "$(db)" ] || [ -z "$(path)" ] || [ -z "$(action)" ]; then \
		echo "Отсутствует параметр db, path или action. Пример: make migrate-action db=chavo_auth_db path=/migrations/auth action=up"; \
		exit 1; \
	fi; \
	docker compose exec -T chavo-postgres psql -U ${POSTGRES_USER} -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$(db)'" | grep -q 1 || \
		docker compose exec -T chavo-postgres psql -U ${POSTGRES_USER} -d postgres -c "CREATE DATABASE $(db)"; \
	docker compose run --rm chavo-postgres-migrate \
		-path "$(path)" \
		-database "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@chavo-postgres:5432/$(db)?sslmode=disable" \
		"$(action)"
