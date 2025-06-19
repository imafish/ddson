#!/bin/bash

base_url="http://10.114.32.120:7777/files/assets"
version="0.0.1-dev"
linux_url="${base_url}/ddson_client_linux_amd64_${version}"
macos_url="${base_url}/ddson_client_darwin_arm64_${version}"

if [[ "$OSTYPE" == "darwin"* ]]; then
  download_url="$macos_url"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
  download_url="$linux_url"
else
  echo "Unsupported OS. Exiting."
  exit 1
fi

set -e
set -x

wget "$download_url" -O ddson_client 
chmod +x ddson_client

server_url="10.114.32.120:5510"
sudo ./ddson_client --stop || true
sudo ./ddson_client --addr "$server_url" --force --daemon
