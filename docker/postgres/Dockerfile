FROM postgres:16

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    postgresql-server-dev-16 \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install pgvector
RUN cd /tmp && \
    git clone --branch v0.7.4 https://github.com/pgvector/pgvector.git && \
    cd pgvector && \
    make && \
    make install && \
    cd / && \
    rm -rf /tmp/pgvector

# Install Apache AGE
RUN cd /tmp && \
    git clone --branch AGE_1_5_0 https://github.com/apache/age.git && \
    cd age && \
    make && \
    make install && \
    cd / && \
    rm -rf /tmp/age

# Cleanup build deps
RUN apt-get purge -y build-essential postgresql-server-dev-16 git && \
    apt-get autoremove -y
