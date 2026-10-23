#!/bin/sh

set -e

export PGPASSWORD=replicator

echo "Waiting for master..."
until pg_isready \
    -h orders-master-postgres \
    -U postgres \
    -d orders-db
do
    sleep 1
done

echo "Master is ready."

if [ ! -s "$PGDATA/PG_VERSION" ]; then

    echo "Creating base backup..."

    rm -rf "$PGDATA"/*
    pg_basebackup \
        -h orders-master-postgres \
        -p 5432 \
        -U replicator \
        -D "$PGDATA" \
        -Fp \
        -Xs \
        -P \
        -R

    echo "Base backup completed."

fi

echo "Starting replica..."

exec docker-entrypoint.sh postgres