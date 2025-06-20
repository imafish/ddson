#!/bin/bash

base_url="http://10.114.32.120:7777/files/assets"
version_url="${base_url}/ddson_client_version.txt"
version=$(wget -qO- "$version_url" | tr -d '[:space:]|\n')
linux_url="${base_url}/ddson_client_linux_amd64_${version}"
macos_url="${base_url}/ddson_client_darwin_arm64_${version}"

server_url="10.114.32.120:5510"

echo "Detected version: $version"

if [[ "$OSTYPE" == "darwin"* ]]; then
  download_url="$macos_url"
  echo "Using macOS client: $download_url"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
  download_url="$linux_url"
  echo "Using Linux client: $download_url"
else
  echo "Unsupported OS. Exiting."
  exit 1
fi

set -e
set -x

current_version="unknown"
if [[ -f "ddson_client" ]]; then
  current_version=$(./ddson_client --version | tr -d '[:space:]|\n')
  if [[ "$current_version" != "$version" ]]; then
    echo "Current version ($current_version) does not match the latest version ($version). Proceeding with update."
    echo "Stopping any existing ddson_client..."
    sudo ./ddson_client --stop || true
    rm -f ddson_client
  fi
else
  echo "No existing ddson_client found."
fi

if [[ "$current_version" != "$version" ]]; then
  echo "Downloading client from: $download_url"
  wget "$download_url" -O ddson_client 
  chmod +x ddson_client

  echo "Starting ddson_client with server address: $server_url"
  sudo ./ddson_client --addr "$server_url" --force --daemon
fi

echo "executing ddson_client command"
./ddson_client --addr "$server_url" --url $@
