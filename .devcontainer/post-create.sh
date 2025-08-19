#!/bin/bash
set -e

echo "Setting up Sprout development environment..."

# Configure git (if not already configured via mount)
if [ ! -f ~/.gitconfig ]; then
    git config --global user.name "Developer"
    git config --global user.email "developer@example.com"
fi

# Install project dependencies
echo "Installing Go dependencies..."
cd /workspaces/sprout
go mod download
go mod tidy

# Run tests to verify setup
echo "Running tests to verify setup..."
go test ./... || echo "No tests found yet - that's OK for initial setup"

# Set up pre-commit hooks if needed
if [ -f .pre-commit-config.yaml ]; then
    echo "Installing pre-commit hooks..."
    pre-commit install || echo "pre-commit not configured"
fi

echo "Dev container setup complete!"
echo "You can now run:"
echo "  go run ./cmd/sprout          - Run the application"
echo "  go test ./...                 - Run tests"
echo "  claude                        - Use Claude Code"