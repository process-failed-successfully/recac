FROM debian:stable-slim

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Install essential tools: sudo, curl, git, ca-certificates
RUN apt-get update && apt-get install -y \
    sudo \
    curl \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Configure sudo for passwordless access by any user (crucial for host UID mapping)
# We allow the "appuser" (which we might not use explicitly, but anyone with sudo can use it)
# More effectively, we allow ALL users to use sudo without password since we map host UID.
RUN echo "ALL ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

WORKDIR /workspace
