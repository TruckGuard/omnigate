#!/bin/bash
set -e

# Use environment variables with defaults if not provided
AUTH_DB_NAME=${AUTH_DB_NAME:-auth_db}
AUTH_USER=${AUTH_USER:-auth_user}
AUTH_PASSWORD=${AUTH_PASSWORD:-auth_password}

CORE_DB_NAME=${CORE_DB_NAME:-core_db}
CORE_USER=${CORE_USER:-core_user}
CORE_PASSWORD=${CORE_PASSWORD:-core_password}

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    -- Create databases
    SELECT 'CREATE DATABASE $AUTH_DB_NAME' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$AUTH_DB_NAME')\gexec
    SELECT 'CREATE DATABASE $CORE_DB_NAME' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$CORE_DB_NAME')\gexec

    -- Create/Update AUTH_USER
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '$AUTH_USER') THEN
            CREATE USER $AUTH_USER WITH PASSWORD '$AUTH_PASSWORD';
        ELSE
            ALTER USER $AUTH_USER WITH PASSWORD '$AUTH_PASSWORD';
        END IF;
    END
    \$\$;

    -- Create/Update CORE_USER
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '$CORE_USER') THEN
            CREATE USER $CORE_USER WITH PASSWORD '$CORE_PASSWORD';
        ELSE
            ALTER USER $CORE_USER WITH PASSWORD '$CORE_PASSWORD';
        END IF;
    END
    \$\$;

    -- Grant privileges
    GRANT ALL PRIVILEGES ON DATABASE $AUTH_DB_NAME TO $AUTH_USER;
    GRANT ALL PRIVILEGES ON DATABASE $CORE_DB_NAME TO $CORE_USER;

    -- Connect to $AUTH_DB_NAME and grant schema privileges
    \c $AUTH_DB_NAME
    GRANT ALL ON SCHEMA public TO $AUTH_USER;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO $AUTH_USER;

    -- Connect to $CORE_DB_NAME and grant schema privileges
    \c $CORE_DB_NAME
    GRANT ALL ON SCHEMA public TO $CORE_USER;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO $CORE_USER;
EOSQL

echo "Databases and users configured successfully."
