#!/usr/bin/env bash
set -euo pipefail

# Script to download/update bundled mermaid.min.js for mdtool

VERSION="${1:-latest}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TARGET_FILE="${ROOT_DIR}/converter/mermaid.min.js"

if [ "${VERSION}" = "latest" ]; then
    URL="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"
else
    # Strip leading 'v' if provided (e.g., v11.4.0 -> 11.4.0)
    VERSION="${VERSION#v}"
    URL="https://cdn.jsdelivr.net/npm/mermaid@${VERSION}/dist/mermaid.min.js"
fi

echo "Downloading mermaid.min.js (${VERSION}) from ${URL}..."

TMP_FILE="$(mktemp)"
trap 'rm -f "${TMP_FILE}"' EXIT

if command -v curl >/dev/null 2>&1; then
    curl -fL "${URL}" -o "${TMP_FILE}"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "${TMP_FILE}" "${URL}"
else
    echo "Error: Neither curl nor wget is installed." >&2
    exit 1
fi

if [ ! -s "${TMP_FILE}" ]; then
    echo "Error: Downloaded file is empty." >&2
    exit 1
fi

mv "${TMP_FILE}" "${TARGET_FILE}"
echo "Successfully updated ${TARGET_FILE}"
