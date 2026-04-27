FROM postgres:16-bookworm

USER root

ARG WALG_VERSION=v3.0.8

RUN apt-get update -qq && \
    apt-get install -y -qq curl cron > /dev/null 2>&1 && \
    curl -fsSL "https://github.com/wal-g/wal-g/releases/download/${WALG_VERSION}/wal-g-pg-22.04-amd64.tar.gz" \
      | tar -xz -C /usr/local/bin/ && \
    mv /usr/local/bin/wal-g-pg-22.04-amd64 /usr/local/bin/wal-g && \
    chmod +x /usr/local/bin/wal-g && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*