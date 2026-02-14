realpath=$(realpath "$0")
script_dir=$(dirname "$realpath")
base_dir=$(dirname "$script_dir")
output_dir="$base_dir/output"

target_dir="bs:workspace_bazel_prefetcher/data/assets"
version_file="$target_dir/ddson_client_version.txt"
tmp_version_file="$output_dir/version.txt"

version=$($output_dir/ddson_client_linux_amd64 --version | tr -d '[:space:]|\n')

set -ex
