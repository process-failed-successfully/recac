FROM golang:1.25-alpine

# Install essential tools
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
    libc6-compat \
    docker-cli \
    coreutils \
    make \
    sudo \
    wget \
    ca-certificates \
    shadow \
    util-linux

# Configure sudo for passwordless access (if needed, though we often run as root)
RUN echo "ALL ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

WORKDIR /workspace