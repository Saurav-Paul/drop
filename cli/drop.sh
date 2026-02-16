#!/bin/bash

# Drop CLI — Encrypted file transfer
# Usage:
#   drop <file>                    Compress + encrypt + upload
#   drop <file> -e 3d             Custom expiry
#   drop <file> -m 5              Max 5 downloads
#   drop <file> --admin           Admin mode (bypass limits)
#   drop get <url>#<key>          Download + decrypt + decompress

set -e

# Configuration
DROP_SERVER="${DROP_SERVER:-http://localhost:8802}"
DROP_ADMIN_USER="${DROP_ADMIN_USER:-}"
DROP_ADMIN_PASS="${DROP_ADMIN_PASS:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

usage() {
    echo -e "${BOLD}Drop — Encrypted file transfer${NC}"
    echo ""
    echo "Usage:"
    echo "  drop <file>                    Compress, encrypt, and upload"
    echo "  drop <file> -e <expiry>        Set expiry (e.g., 30m, 2h, 3d, 1w)"
    echo "  drop <file> -m <max>           Set max downloads"
    echo "  drop <file> --admin            Use admin credentials"
    echo "  drop get <url>#<key>           Download, decrypt, and decompress"
    echo ""
    echo "Environment:"
    echo "  DROP_SERVER      Server URL (default: http://localhost:8802)"
    echo "  DROP_ADMIN_USER  Admin username (for --admin mode)"
    echo "  DROP_ADMIN_PASS  Admin password (for --admin mode)"
    exit 0
}

die() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

# Check dependencies
check_deps() {
    for cmd in curl gzip openssl; do
        command -v "$cmd" &>/dev/null || die "$cmd is required but not installed"
    done
}

# Upload: compress + encrypt + upload
do_upload() {
    local file="$1"
    shift

    [ -f "$file" ] || die "File not found: $file"

    local expiry=""
    local max_downloads=""
    local admin=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            -e|--expiry) expiry="$2"; shift 2 ;;
            -m|--max-downloads) max_downloads="$2"; shift 2 ;;
            --admin) admin=true; shift ;;
            -h|--help) usage ;;
            *) die "Unknown option: $1" ;;
        esac
    done

    local filename
    filename=$(basename "$file")

    echo -e "${CYAN}Compressing...${NC}"
    local tmpfile
    tmpfile=$(mktemp)
    gzip -c "$file" > "$tmpfile"

    # Generate random 256-bit key
    local key
    key=$(openssl rand -hex 32)

    echo -e "${CYAN}Encrypting...${NC}"
    local encrypted
    encrypted=$(mktemp)
    openssl enc -aes-256-cbc -pbkdf2 -iter 100000 \
        -in "$tmpfile" -out "$encrypted" \
        -pass "pass:$key" 2>/dev/null

    rm -f "$tmpfile"

    # Build headers
    local headers=()
    [ -n "$expiry" ] && headers+=(-H "X-Expires: $expiry")
    [ -n "$max_downloads" ] && headers+=(-H "X-Max-Downloads: $max_downloads")

    if [ "$admin" = true ]; then
        [ -z "$DROP_ADMIN_USER" ] && die "DROP_ADMIN_USER not set"
        [ -z "$DROP_ADMIN_PASS" ] && die "DROP_ADMIN_PASS not set"
        headers+=(-H "X-Admin-User: $DROP_ADMIN_USER")
        headers+=(-H "X-Admin-Pass: $DROP_ADMIN_PASS")
    fi

    echo -e "${CYAN}Uploading...${NC}"
    local response
    response=$(curl -sS -T - \
        "${headers[@]}" \
        "${DROP_SERVER}/${filename}" \
        < "$encrypted")

    rm -f "$encrypted"

    # Parse response
    local url
    url=$(echo "$response" | grep -o '"url":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ -z "$url" ]; then
        die "Upload failed: $response"
    fi

    local file_size
    file_size=$(echo "$response" | grep -o '"size":[0-9]*' | head -1 | cut -d: -f2)

    echo ""
    echo -e "${GREEN}${BOLD}Upload complete!${NC}"
    echo -e "${BOLD}URL:${NC} ${url}#${key}"
    echo ""
    echo -e "${YELLOW}Share the full URL (including the #key part).${NC}"
    echo -e "${YELLOW}The key never leaves your machine — the server only stores encrypted data.${NC}"

    # Copy to clipboard if possible
    if command -v pbcopy &>/dev/null; then
        echo -n "${url}#${key}" | pbcopy
        echo -e "${GREEN}Copied to clipboard.${NC}"
    elif command -v xclip &>/dev/null; then
        echo -n "${url}#${key}" | xclip -selection clipboard
        echo -e "${GREEN}Copied to clipboard.${NC}"
    fi
}

# Download: download + decrypt + decompress
do_download() {
    local full_url="$1"

    [ -z "$full_url" ] && die "Usage: drop get <url>#<key>"

    # Split URL and key at the fragment
    local url key
    url="${full_url%%#*}"
    key="${full_url##*#}"

    [ "$url" = "$full_url" ] && die "No encryption key found in URL (expected #key fragment)"
    [ -z "$key" ] && die "Empty encryption key"

    # Extract filename from URL
    local filename
    filename=$(basename "$url")

    echo -e "${CYAN}Downloading...${NC}"
    local encrypted
    encrypted=$(mktemp)
    local http_code
    http_code=$(curl -sS -w "%{http_code}" -o "$encrypted" "$url")

    if [ "$http_code" != "200" ]; then
        rm -f "$encrypted"
        die "Download failed (HTTP $http_code)"
    fi

    echo -e "${CYAN}Decrypting...${NC}"
    local compressed
    compressed=$(mktemp)
    if ! openssl enc -aes-256-cbc -pbkdf2 -iter 100000 -d \
        -in "$encrypted" -out "$compressed" \
        -pass "pass:$key" 2>/dev/null; then
        rm -f "$encrypted" "$compressed"
        die "Decryption failed — wrong key?"
    fi

    rm -f "$encrypted"

    echo -e "${CYAN}Decompressing...${NC}"
    local output="$filename"

    # Avoid overwriting
    if [ -f "$output" ]; then
        local base ext counter=1
        if [[ "$output" == *.* ]]; then
            base="${output%.*}"
            ext=".${output##*.}"
        else
            base="$output"
            ext=""
        fi
        while [ -f "$output" ]; do
            output="${base}_${counter}${ext}"
            counter=$((counter + 1))
        done
    fi

    if ! gzip -d -c "$compressed" > "$output" 2>/dev/null; then
        rm -f "$compressed"
        die "Decompression failed"
    fi

    rm -f "$compressed"

    echo ""
    echo -e "${GREEN}${BOLD}Download complete!${NC}"
    echo -e "${BOLD}Saved:${NC} $output"
}

# Main
check_deps

case "${1:-}" in
    -h|--help|"")
        usage
        ;;
    get)
        shift
        do_download "$@"
        ;;
    *)
        do_upload "$@"
        ;;
esac
