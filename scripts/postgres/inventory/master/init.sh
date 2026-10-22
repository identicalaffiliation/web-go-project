#!/bin/sh

set -e

echo "Configuring PostgreSQL primary..."

cat >> "$PGDATA/postgresql.conf" << \
EOF
wal_level = replica
max_wal_senders = 10
max_replication_slots = 10

# Helpful for replication
wal_keep_size = 128MB
EOF

cat >> "$PGDATA/pg_hba.conf" << \
EOF
host replication replicator 0.0.0.0/0 scram-sha-256
EOF

psql -v ON_ERROR_STOP=1 \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" <<-EOSQL

    CREATE ROLE replicator
      WITH REPLICATION
      LOGIN
      PASSWORD 'replicator';
EOSQL

echo "Master node configured"