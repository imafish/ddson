#!/bin/bash


realpath=$(realpath "$0")
script_dir=$(dirname "$realpath")
base_dir=$(dirname "$script_dir")
output_dir="$base_dir/output"

target_dir="$HOME/workspace_bazel_prefetcher/data/assets"
version=0.0.1-dev

cp "$output_dir/ddson_client_linux_amd64" "$target_dir/ddson_client_linux_amd64_$version"
cp "$output_dir/ddson_client_darwin_arm64" "$target_dir/ddson_client_darwin_arm64_$version"
# Windows cannot forkexec
# cp "$output_dir/ddson_client_windows_amd64.exe" "$target_dir/ddson_client_windows_amd64_$version.exe"
cp "$base_dir/scripts/upgrade.sh" "$target_dir/ddson_client_upgrade.sh"
