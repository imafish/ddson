#!/bin/bash

source "$(dirname "$0")/common.sh"

echo "version: $version"

scp "$output_dir/ddson_client_linux_amd64" "$target_dir/ddson_client_linux_amd64_$version"
scp "$output_dir/ddson_client_darwin_arm64" "$target_dir/ddson_client_darwin_arm64_$version"
# Windows cannot forkexec
# cp "$output_dir/ddson_client_windows_amd64.exe" "$target_dir/ddson_client_windows_amd64_$version.exe"
scp "$base_dir/scripts/upgrade.sh" "$target_dir/ddson_client_upgrade.sh"

echo $version > "$tmp_version_file"
scp "$tmp_version_file" "$version_file"
rm "$tmp_version_file"

echo "Files moved to $target_dir"
echo "Version file updated to $version_file, version: $version"
