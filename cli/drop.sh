#!/bin/bash

# Drop CLI — Encrypted file transfer
# Usage:
#   drop <file>                    Compress + encrypt + upload (prompts for password)
#   drop <file> -e 3d             Custom expiry
#   drop <file> -m 5              Max 5 downloads
#   drop <file> --admin           Admin mode (bypass limits)
#   drop get <url>                Download + decrypt + decompress (prompts for password)

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
    echo "  drop get <url>                 Download, decrypt, and decompress"
    echo ""
    echo "Environment:"
    echo "  DROP_SERVER      Server URL (default: http://localhost:8802)"
    echo "  DROP_ADMIN_USER  Admin username (optional, prompts if not set)"
    echo "  DROP_ADMIN_PASS  Admin password (optional, prompts if not set)"
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

# Prompt for password (hidden input, confirm on encrypt)
# Returns empty string if user presses Enter (no encryption)
ask_password() {
    local mode="$1" # "encrypt" or "decrypt"

    echo -ne "${BOLD}Password (empty = no encryption): ${NC}" >&2
    read -s password
    echo "" >&2

    if [ -n "$password" ] && [ "$mode" = "encrypt" ]; then
        echo -ne "${BOLD}Confirm:  ${NC}" >&2
        read -s password_confirm
        echo "" >&2
        [ "$password" != "$password_confirm" ] && die "Passwords don't match"
    fi

    echo "$password"
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

    # Ask for password
    local password
    password=$(ask_password encrypt)

    echo -e "${CYAN}Compressing...${NC}"
    local compressed
    compressed=$(mktemp)
    gzip -c "$file" > "$compressed"

    local upload_file="$compressed"
    if [ -n "$password" ]; then
        echo -e "${CYAN}Encrypting...${NC}"
        local encrypted
        encrypted=$(mktemp)
        openssl enc -aes-256-cbc -pbkdf2 -iter 100000 \
            -in "$compressed" -out "$encrypted" \
            -pass "pass:$password" 2>/dev/null
        rm -f "$compressed"
        upload_file="$encrypted"
    fi

    # Build headers
    local headers=()
    [ -n "$expiry" ] && headers+=(-H "X-Expires: $expiry")
    [ -n "$max_downloads" ] && headers+=(-H "X-Max-Downloads: $max_downloads")

    if [ "$admin" = true ]; then
        local admin_user="${DROP_ADMIN_USER:-}"
        local admin_pass="${DROP_ADMIN_PASS:-}"
        if [ -z "$admin_user" ]; then
            echo -ne "${BOLD}Admin user: ${NC}" >&2
            read admin_user
            [ -z "$admin_user" ] && die "Admin user cannot be empty"
        fi
        if [ -z "$admin_pass" ]; then
            echo -ne "${BOLD}Admin pass: ${NC}" >&2
            read -s admin_pass
            echo "" >&2
            [ -z "$admin_pass" ] && die "Admin pass cannot be empty"
        fi
        headers+=(-H "X-Admin-User: $admin_user")
        headers+=(-H "X-Admin-Pass: $admin_pass")
    fi

    echo -e "${CYAN}Uploading...${NC}"
    local response
    response=$(curl -sS -T - \
        "${headers[@]}" \
        "${DROP_SERVER}/${filename}" \
        < "$upload_file")

    rm -f "$upload_file"

    # Parse response
    local url
    url=$(echo "$response" | grep -o '"url":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ -z "$url" ]; then
        die "Upload failed: $response"
    fi

    echo ""
    echo -e "${GREEN}${BOLD}Upload complete!${NC}"
    echo -e "${BOLD}URL:${NC} ${url}"
    echo ""
    if [ -n "$password" ]; then
        echo -e "${YELLOW}The recipient will need the password to decrypt.${NC}"
    else
        echo -e "${YELLOW}No encryption — file is compressed only.${NC}"
    fi

    # Copy to clipboard if possible
    if command -v pbcopy &>/dev/null; then
        echo -n "${url}" | pbcopy
        echo -e "${GREEN}URL copied to clipboard.${NC}"
    elif command -v xclip &>/dev/null; then
        echo -n "${url}" | xclip -selection clipboard
        echo -e "${GREEN}URL copied to clipboard.${NC}"
    fi
}

# Download: download + decrypt + decompress
do_download() {
    local url="$1"

    [ -z "$url" ] && die "Usage: drop get <url>"

    # Strip any trailing fragment just in case
    url="${url%%#*}"

    # Extract filename from URL
    local filename
    filename=$(basename "$url")

    # Ask for password
    local password
    password=$(ask_password decrypt)

    echo -e "${CYAN}Downloading...${NC}"
    local downloaded
    downloaded=$(mktemp)
    local http_code
    http_code=$(curl -sSL -w "%{http_code}" -o "$downloaded" "$url")

    if [ "$http_code" != "200" ]; then
        rm -f "$downloaded"
        die "Download failed (HTTP $http_code)"
    fi

    local compressed
    if [ -n "$password" ]; then
        echo -e "${CYAN}Decrypting...${NC}"
        compressed=$(mktemp)
        if ! openssl enc -aes-256-cbc -pbkdf2 -iter 100000 -d \
            -in "$downloaded" -out "$compressed" \
            -pass "pass:$password" 2>/dev/null; then
            rm -f "$downloaded" "$compressed"
            die "Decryption failed — wrong password?"
        fi
        rm -f "$downloaded"
    else
        compressed="$downloaded"
    fi

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
