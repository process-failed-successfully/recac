ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS base

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
    docker-cli

# Configure NPM mirror
# RUN npm config set registry https://registry.npmmirror.com/

# Install Gemini CLI
RUN npm install -g @google/gemini-cli

# Install Cursor Agent
ENV HOME=/root
RUN curl -fsS https://cursor.com/install | bash
ENV PATH="${HOME}/.local/bin:${PATH}"

WORKDIR /app

# Download Utils
FROM base AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -buildvcs=false -o recac ./cmd/recac
RUN go build -buildvcs=false -o agent-bridge ./cmd/agent-bridge

# Production Image
FROM base AS production
WORKDIR /app
COPY --from=builder /app/recac /usr/local/bin/recac
COPY --from=builder /app/agent-bridge /usr/local/bin/agent-bridge

# Default entrypoint
CMD ["recac"]
