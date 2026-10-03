#!/bin/bash
set -e

# Функция создания базы данных, если она еще не существует
create_database() {
    local db=$1
    if [ -n "$db" ]; then
        echo "Checking/creating database: $db"
        psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
            SELECT 'CREATE DATABASE $db'
            WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
EOSQL
    fi
}

create_database "$AUTH_DB_NAME"
create_database "$USER_DB_NAME"
create_database "$CONVERSATION_DB_NAME"
