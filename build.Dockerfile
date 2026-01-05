FROM golang:1.24-alpine

WORKDIR /app

# Configure Alpine mirror for China (User in +08:00)
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

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

# No default command - source is mounted at runtime
