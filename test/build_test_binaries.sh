#!/bin/bash

# Build test binaries for integration testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/test/bin"

echo "Building test binaries..."
echo "Project root: $PROJECT_ROOT"
echo "Output directory: $OUTPUT_DIR"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build ddson_client for testing
echo "Building ddson_client..."
cd "$PROJECT_ROOT"
go build -o "$OUTPUT_DIR/ddson_client" ./cmd/client

# Build ddson_server for testing
echo "Building ddson_server..."
go build -o "$OUTPUT_DIR/ddson_server" ./cmd/server

echo "Build complete!"
echo "Binaries located in: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"
