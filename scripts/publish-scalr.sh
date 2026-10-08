#!/usr/bin/env bash
# Publishes a GitHub release to the Scalr provider registry, from an IP Scalr allows.
# Usage: op run --environment <env> -- scripts/publish-scalr.sh v1.2.3
set -euo pipefail

tag=${1:?usage: $0 <tag>}
: "${SCALR_HOSTNAME:?}" "${SCALR_TOKEN:?}"
provider_id=${SCALR_PROVIDER_ID:-prv-v0pehvboj3ijlqita}
gpg_key_id=${SCALR_GPG_KEY_ID:-gpg-v0pehvbmm4qpevs9v}
api="https://${SCALR_HOSTNAME}/api/iacp/v3"

call() {
  local body
  body=$(curl -sS --fail-with-body -H "Authorization: Bearer ${SCALR_TOKEN}" "$@") || { echo "${*: -1}: ${body}" >&2; return 1; }
  printf '%s\n' "$body"
}

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
gh release download "$tag" -R lemon-markets/terraform-provider-jiraassets -p release.tar.gz -D "$dir"

version=$(jq -n --arg v "${tag#v}" --arg p "$provider_id" --arg g "$gpg_key_id" '{data: {
    type: "provider-versions",
    attributes: {version: $v},
    relationships: {
      provider: {data: {type: "providers", id: $p}},
      "gpg-key": {data: {type: "gpg-keys", id: $g}}}}}' |
  call -X POST -H 'Content-Type: application/vnd.api+json' --data @- "${api}/provider-versions")
id=$(jq -r .data.id <<<"$version")
upload=$(jq -r .data.links.upload <<<"$version")

curl -fsS -X PUT --upload-file "$dir/release.tar.gz" "$upload"

for _ in $(seq 60); do
  attrs=$(call "${api}/provider-versions/${id}" | jq .data.attributes)
  case $(jq -r .status <<<"$attrs") in
    uploaded) echo "Published ${tag} as ${id}"; exit 0 ;;
    errored) jq -r '."error-message"' <<<"$attrs" >&2; exit 1 ;;
  esac
  sleep 5
done
echo "${id} still pending after 5 minutes" >&2; exit 1
