#!/bin/bash

realpath=$(realpath "$0")
script_dir=$(dirname "$realpath")
base_dir=$(dirname "$script_dir")

output_dir="$base_dir/output"
mkdir -p "$output_dir"

client_dir="$base_dir/cmd/client"
server_dir="$base_dir/cmd/server"

# Build the client
client_linux_amd64="$output_dir/ddson_client_linux_amd64"
client_darwin_arm64="$output_dir/ddson_client_darwin_arm64"
pushd "$client_dir" || exit 1
GOOS=linux GOARCH=amd64 go build -o "$client_linux_amd64" 
GOOS=darwin GOARCH=arm64 go build -o "$client_darwin_arm64" 
popd || exit 1

# Build the server
server_linux_amd64="$output_dir/ddson_server_linux_amd64"
pushd "$server_dir" || exit 1
GOOS=linux GOARCH=amd64 go build -o "$server_linux_amd64" 
popd || exit 1
