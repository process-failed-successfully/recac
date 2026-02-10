FROM golang:1.25

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
    sudo \
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Allow pip to install global packages (Debian 12 PEP 668)
ENV PIP_BREAK_SYSTEM_PACKAGES=1

# Install Gemini CLI
RUN npm install -g @google/gemini-cli

# Install OpenCode CLI
RUN npm install -g opencode-ai --ignore-scripts

# Install Cursor Agent
ENV HOME=/root
RUN curl -fsS https://cursor.com/install | bash
ENV PATH="${HOME}/.local/bin:${PATH}"

# Configure sudo for passwordless access (if needed, though we often run as root)
RUN echo "ALL ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

WORKDIR /workspace
