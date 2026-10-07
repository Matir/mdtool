#!/usr/bin/env bash
set -euo pipefail

# Script to download/update bundled mathjax.min.js (tex-svg.js) for mdtool

VERSION="${1:-3}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
TARGET_FILE="${ROOT_DIR}/converter/mathjax.min.js"

# Strip leading 'v' if provided (e.g., v3.2.2 -> 3.2.2)
VERSION="${VERSION#v}"
URL="https://cdn.jsdelivr.net/npm/mathjax@${VERSION}/es5/tex-svg.js"

echo "Downloading mathjax.min.js (${VERSION}) from ${URL}..."

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
