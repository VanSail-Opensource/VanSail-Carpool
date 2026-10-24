FROM postgres:18-bookworm
RUN apt-get update \
    && apt-get install -y --no-install-recommends pgbackrest \
    && install -d -o postgres -g postgres /var/lib/pgbackrest /var/log/pgbackrest \
    && rm -rf /var/lib/apt/lists/*
