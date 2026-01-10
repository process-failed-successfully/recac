ARG GO_VERSION=1.25
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION} AS base

# Install essential tools
RUN apt-get update && apt-get install -y \
    nodejs \
    npm \
    python3 \
    python3-pip \
    curl \
    git \
    jq \
    unzip \
    docker.io \
    make \
    && rm -rf /var/lib/apt/lists/*

# Configure NPM mirror
# RUN npm config set registry https://registry.npmmirror.com/

# Install Gemini CLI
RUN npm install -g @google/gemini-cli

# Install OpenCode CLI
RUN npm install -g opencode-ai --ignore-scripts

# Install Cursor Agent
ENV HOME=/root
RUN curl -fsS https://cursor.com/install | bash
ENV PATH="${HOME}/.local/bin:${PATH}"

WORKDIR /app

# Download Utils
FROM base AS builder
ARG CACHE_BYPASS=unknown
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -buildvcs=false -o recac ./cmd/recac
RUN go build -buildvcs=false -o orchestrator ./cmd/orchestrator
RUN go build -buildvcs=false -o recac-agent ./cmd/agent
RUN go build -buildvcs=false -o agent-bridge ./cmd/agent-bridge

# Production Image
FROM base AS production
WORKDIR /app
COPY --from=builder /app/recac /usr/local/bin/recac
COPY --from=builder /app/orchestrator /usr/local/bin/orchestrator
COPY --from=builder /app/recac-agent /usr/local/bin/recac-agent
COPY --from=builder /app/agent-bridge /usr/local/bin/agent-bridge

# Default entrypoint
CMD ["recac"]
