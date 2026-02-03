FROM oven/bun:1 AS base

# Install git and common tools
RUN apt-get update && apt-get install -y \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create test user with home directory
RUN useradd -m -s /bin/bash testuser
USER testuser
WORKDIR /home/testuser

# Initialize git config (required for commits)
RUN git config --global user.email "test@example.com" && \
    git config --global user.name "Test User" && \
    git config --global init.defaultBranch main

# Create bin directory for paw
RUN mkdir -p /home/testuser/.local/bin
ENV PATH="/home/testuser/.local/bin:${PATH}"
