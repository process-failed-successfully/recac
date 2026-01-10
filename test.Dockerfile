ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine

WORKDIR /app


# Install system dependencies
# nodejs/npm for gemini-cli
# python3/pip
# curl/git/jq/unzip/bash for utilities
# libc6-compat for potential glibc compatibility (Cursor CLI)
RUN apk add --no-cache \
    nodejs \
    npm \
    python3 \
    py3-pip \
    curl \
    git \
    jq \
    bash \
    unzip \
    docker-cli \
    libc6-compat

# Configure NPM mirror
RUN npm config set registry https://registry.npmmirror.com/

# Install Gemini CLI
RUN npm install -g @google/gemini-cli

# Install Cursor Agent
# Script installs to ~/.local/bin
ENV HOME=/root
# Cursor install might still be slow if not mirrored, but let's try
RUN curl -fsS https://cursor.com/install | bash
ENV PATH="${HOME}/.local/bin:${PATH}"

# Copy and configure the entrypoint
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT ["docker-entrypoint.sh"]

# No default command - source is mounted at runtime
