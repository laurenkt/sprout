FROM golang:1.24.2-bookworm

# Install basic development tools
RUN apt-get update && apt-get install -y \
    git \
    curl \
    vim \
    sudo \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Install Claude Code
RUN curl -fsSL https://github.com/anthropics/claude-code/releases/latest/download/install.sh | sh

# Install Go tools
RUN go install -v golang.org/x/tools/gopls@latest \
    && go install -v github.com/go-delve/delve/cmd/dlv@latest \
    && go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@latest \
    && go install -v golang.org/x/tools/cmd/goimports@latest \
    && go install -v github.com/cucumber/godog/cmd/godog@latest

# Set GOPATH
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

# Switch to non-root user
USER $USERNAME

# Set the default shell to bash
SHELL ["/bin/bash", "-c"]