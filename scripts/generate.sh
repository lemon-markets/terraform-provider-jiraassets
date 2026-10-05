#!/usr/bin/env bash
# Regenerate docs/ from the provider schema and the configs in examples/.
#
# tfplugindocs can build the provider and call a CLI itself, but only ever
# terraform: it hardcodes registry.terraform.io in the plugin mirror path it
# writes and in the schema key it reads back, and downloads a binary when none
# is on PATH. Export the schema with tofu instead and hand it over via
# -providers-schema, which skips both the build and the CLI call.
#
# Requires tofu on PATH. Set TOFU to override.
set -euo pipefail

TOFU="${TOFU:-tofu}"
PROVIDER_NAME="jiraassets"
# tfplugindocs looks the schema up under this address, so declare it verbatim
# below: tofu keys the output by whatever source address the config asks for.
PROVIDER_ADDR="registry.terraform.io/hashicorp/${PROVIDER_NAME}"

cd "$(dirname "$0")/.."
repo="$PWD"

if ! command -v "$TOFU" >/dev/null 2>&1; then
    echo "generate.sh: $TOFU not found on PATH" >&2
    exit 1
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

# A filesystem mirror the provider can be "installed" from, so init needs no
# network and no registry entry for a provider that is not published yet.
plugin_dir="$workdir/plugins/${PROVIDER_ADDR}/0.0.1/$(go env GOOS)_$(go env GOARCH)"
mkdir -p "$plugin_dir"
go build -o "$plugin_dir/terraform-provider-${PROVIDER_NAME}" .

cat > "$workdir/provider.tf" <<EOF
terraform {
  required_providers {
    ${PROVIDER_NAME} = {
      source = "${PROVIDER_ADDR}"
    }
  }
}
EOF

(
    cd "$workdir"
    "$TOFU" init -no-color -input=false -get=false -plugin-dir=./plugins >/dev/null
    "$TOFU" providers schema -json > schema.json
)

go run -tags tools github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate \
    --provider-name "terraform-provider-${PROVIDER_NAME}" \
    --providers-schema "$workdir/schema.json" \
    --provider-dir "$repo"
