FROM golang:1.24-bookworm

ARG TZ
ENV TZ="$TZ"

ARG CLAUDE_CODE_VERSION=latest

# Install basic development tools
RUN apt-get update && apt-get install -y --no-install-recommends \
  less \
  git \
  procps \
  sudo \
  fzf \
  zsh \
  man-db \
  unzip \
  gnupg2 \
  gh \
  jq \
  nano \
  vim \
  curl \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

# Create node user to match the Anthropic sample
RUN groupadd --gid 1000 node \
  && useradd --uid 1000 --gid node --shell /bin/bash --create-home node \
  && echo node ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/node \
  && chmod 0440 /etc/sudoers.d/node

# Ensure node user has access to /usr/local/share
RUN mkdir -p /usr/local/share/npm-global && \
  chown -R node:node /usr/local/share

ARG USERNAME=node

# Persist bash history
RUN SNIPPET="export PROMPT_COMMAND='history -a' && export HISTFILE=/commandhistory/.bash_history" \
  && mkdir /commandhistory \
  && touch /commandhistory/.bash_history \
  && chown -R $USERNAME /commandhistory \
  && echo "$SNIPPET" >> "/home/$USERNAME/.bashrc"

# Set `DEVCONTAINER` environment variable to help with orientation
ENV DEVCONTAINER=true

# Create workspace and config directories and set permissions
RUN mkdir -p /workspace /home/node/.claude && \
  chown -R node:node /workspace /home/node/.claude

WORKDIR /workspace

# Install Claude Code to /usr/local/bin
RUN if [ "$CLAUDE_CODE_VERSION" = "latest" ]; then \
    curl -fsSL https://github.com/anthropics/claude-code/releases/latest/download/install.sh | sh -s -- --install-dir /usr/local/bin; \
  else \
    curl -fsSL https://github.com/anthropics/claude-code/releases/download/v${CLAUDE_CODE_VERSION}/install.sh | sh -s -- --install-dir /usr/local/bin; \
  fi \
  && chmod 755 /usr/local/bin/claude

# Install Go tools
RUN go install -v golang.org/x/tools/gopls@latest \
  && go install -v github.com/go-delve/delve/cmd/dlv@latest \
  && go install -v golang.org/x/tools/cmd/goimports@latest \
  && go install -v github.com/cucumber/godog/cmd/godog@latest

# Switch to node user
USER node

# Ensure PATH includes Claude Code
ENV PATH="/usr/local/bin:$PATH"

# Configure git delta
RUN git config --global core.pager delta \
  && git config --global interactive.diffFilter "delta --color-only" \
  && git config --global delta.light false

# Verify Claude Code installation
RUN claude --version || echo "Claude Code not found in PATH"