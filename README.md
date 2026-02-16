# Drop

Self-hosted encrypted file transfer. A [transfer.sh](https://transfer.sh) alternative with end-to-end encryption.

The server stores encrypted blobs. It never sees your plaintext. Files are uploaded via streaming `PUT`, downloaded via `GET`, and automatically cleaned up when they expire.

---

## Quick Start

### 1. Start the server

```bash
git clone https://github.com/Saurav-Paul/drop.git
cd drop
docker compose up -d
```

The server runs at `http://localhost:8802`. Default admin credentials: `admin`/`admin` (set via env vars).

### 2. Upload a file

**With Drop CLI** (encrypts automatically):

```bash
./cli/drop.sh secret.pdf
# Password: ****
# Confirm:  ****
# Upload complete!
# URL: http://localhost:8802/aB3xYz/secret.pdf
```

**With curl** (no encryption):

```bash
curl -T secret.pdf http://localhost:8802/secret.pdf
# {"url":"http://localhost:8802/aB3xYz/secret.pdf","code":"aB3xYz",...}
```

### 3. Download

**With Drop CLI** (decrypts automatically):

```bash
./cli/drop.sh get http://localhost:8802/aB3xYz/secret.pdf
# Password: ****
# Download complete!
# Saved: secret.pdf
```

**With curl**:

```bash
curl -O http://localhost:8802/aB3xYz/secret.pdf
```

---

## Server

### Docker (recommended)

```bash
docker compose up -d
```

The `docker-compose.yml` binds `./data` to `/data` inside the container. Your database and uploaded files persist there.

**Environment variables:**

| Variable | Default | Description |
|---|---|---|
| `DROP_ADMIN_USER` | `admin` | Admin username |
| `DROP_ADMIN_PASS` | `admin` | Admin password |
| `DATA_DIR` | `/data` | Storage directory inside the container |

**Production** — uses a pre-built image instead of building from source:

```bash
docker compose -f docker-compose.prod.yml up -d
```

### Local development

```bash
uv venv && uv pip install fastapi "uvicorn[standard]" sqlalchemy alembic jinja2 python-multipart pydantic
PYTHONPATH=backend .venv/bin/uvicorn backend.main:app --port 8802 --reload
```

### Cleanup

A cron job runs inside the container every 12 hours to:

- Delete files past their `expires_at` time
- Delete files that have reached `max_downloads`
- Remove orphaned directories on disk

Expired files are also blocked from download immediately — the cron just reclaims disk space.

---

## Drop CLI

Standalone bash script that handles compression, encryption, and upload in one command. Requires only `curl`, `gzip`, and `openssl` (pre-installed on macOS and most Linux distros).

### Installation

**Via tooldock:**

```bash
tooldock install drop
tooldock drop myfile.txt
```

**Standalone:**

```bash
# From the drop repo
chmod +x cli/drop.sh
alias drop="./cli/drop.sh"
```

### Upload

```bash
drop myfile.txt
```

You'll be prompted for a password (entered twice for confirmation). The file is compressed with gzip, encrypted with AES-256-CBC (PBKDF2, 100k iterations), and uploaded. The server only receives the encrypted blob.

**Options:**

```bash
drop myfile.txt -e 3d          # Expires in 3 days
drop myfile.txt -m 5           # Max 5 downloads
drop myfile.txt -e 1w -m 10   # Both
drop myfile.txt --admin        # Admin mode (bypass server limits)
```

The URL is automatically copied to your clipboard (macOS/Linux).

### Download

```bash
drop get http://localhost:8802/aB3xYz/myfile.txt
```

You'll be prompted for the password. The file is downloaded, decrypted, and decompressed. If a file with the same name already exists, it's saved as `myfile_1.txt`, `myfile_2.txt`, etc.

### Environment

| Variable | Default | Description |
|---|---|---|
| `DROP_SERVER` | `http://localhost:8802` | Server URL |
| `DROP_ADMIN_USER` | — | Admin username (for `--admin` flag) |
| `DROP_ADMIN_PASS` | — | Admin password (for `--admin` flag) |

### How encryption works

```
Upload:  file → gzip → AES-256-CBC (password + PBKDF2) → upload encrypted blob
Download: encrypted blob → AES-256-CBC decrypt → gunzip → file
```

- Encryption uses OpenSSL's `aes-256-cbc` with PBKDF2 key derivation (100,000 iterations)
- The password never leaves your machine
- The server stores and serves only the encrypted blob
- Without the password, the file is unreadable

---

## Bare Usage (curl)

No CLI needed. Use curl directly for uploads and downloads. Files are stored as-is (no encryption unless you handle it yourself).

### Upload

```bash
# Basic upload
curl -T myfile.txt http://localhost:8802/myfile.txt

# With expiry
curl -T myfile.txt -H "X-Expires: 3d" http://localhost:8802/myfile.txt

# With max downloads
curl -T myfile.txt -H "X-Max-Downloads: 5" http://localhost:8802/myfile.txt

# Both
curl -T myfile.txt -H "X-Expires: 1w" -H "X-Max-Downloads: 10" http://localhost:8802/myfile.txt

# Admin mode (bypass size/expiry limits)
curl -T myfile.txt \
  -H "X-Admin-User: admin" \
  -H "X-Admin-Pass: admin" \
  http://localhost:8802/myfile.txt
```

**Expiry formats:** `30m` (minutes), `2h` (hours), `3d` (days), `1w` (weeks)

**Response:**

```json
{
  "url": "http://localhost:8802/aB3xYz/myfile.txt",
  "code": "aB3xYz",
  "filename": "myfile.txt",
  "size": 1024,
  "expires_at": "2026-02-19T12:00:00Z",
  "max_downloads": 5
}
```

### Download

```bash
curl -O http://localhost:8802/aB3xYz/myfile.txt
```

Returns the raw file with `Content-Disposition: attachment` header. Returns `404` if expired or max downloads reached.

### DIY encryption with curl

If you want encryption without the CLI:

```bash
# Upload: compress + encrypt + upload
gzip -c secret.txt | openssl enc -aes-256-cbc -pbkdf2 -iter 100000 -pass pass:mypassword | \
  curl -T - http://localhost:8802/secret.txt

# Download: download + decrypt + decompress
curl -s http://localhost:8802/aB3xYz/secret.txt | \
  openssl enc -aes-256-cbc -pbkdf2 -iter 100000 -d -pass pass:mypassword | \
  gunzip > secret.txt
```

---

## Admin Dashboard

Access the web UI at `http://localhost:8802/` with admin credentials passed as headers. The dashboard shows:

- All uploaded files with codes, sizes, download counts, and expiry times
- Storage statistics
- Delete buttons (per file, via HTMX)

The settings page at `http://localhost:8802/settings` lets you configure:

| Setting | Default | Description |
|---|---|---|
| Default Expiry | `24h` | Applied when no `X-Expires` header is sent |
| Max Expiry | `7d` | Maximum expiry for non-admin uploads |
| Max File Size | `100MB` | Per-file size limit for non-admin uploads |
| Storage Limit | `1GB` | Total storage limit across all files |
| Upload API Key | — | If set, uploads require `X-Upload-Key` header |

Admin uploads (with `X-Admin-User`/`X-Admin-Pass` headers) bypass all limits.

---

## API Reference

### Upload

```
PUT /{filename}
```

| Header | Required | Description |
|---|---|---|
| `X-Expires` | No | Expiry duration (`30m`, `2h`, `3d`, `1w`) |
| `X-Max-Downloads` | No | Max download count |
| `X-Admin-User` | No | Admin username (bypass limits) |
| `X-Admin-Pass` | No | Admin password |
| `X-Upload-Key` | No | Upload API key (if configured in settings) |

**Response:** `200` with JSON body containing `url`, `code`, `filename`, `size`, `expires_at`, `max_downloads`.

### Download

```
GET /{code}/{filename}
```

**Response:** `200` with file stream, or `404` if not found / expired / max downloads reached.

### List files

```
GET /api/files
```

Requires admin headers. Returns JSON array of all files.

### Get file info

```
GET /api/files/{code}
```

Requires admin headers. Returns JSON object for a single file.

### Delete file

```
DELETE /api/files/{code}
```

Requires admin headers. Returns `204` on success.

### Get settings

```
GET /api/settings
```

Requires admin headers.

### Update settings

```
PUT /api/settings
Content-Type: application/json

{"default_expiry": "12h", "max_file_size": "500MB"}
```

Requires admin headers. Partial updates — only include fields you want to change.

### Health check

```
GET /api/health
```

Returns `{"status": "ok"}`. No authentication required.

---

## Project Structure

```
drop/
├── backend/
│   ├── api/
│   │   ├── download/      # GET /{code}/{filename}
│   │   ├── upload/         # PUT /{filename}
│   │   ├── files/          # /api/files CRUD
│   │   ├── settings/       # /api/settings CRUD
│   │   └── pages/          # Dashboard + settings HTML
│   ├── db_migrations/      # Alembic migrations
│   ├── orm/                # Central model imports
│   ├── main.py             # FastAPI app entry point
│   ├── config.py           # Environment-based configuration
│   ├── database.py         # SQLAlchemy setup (SQLite)
│   ├── auth.py             # Admin header validation
│   └── cleanup.py          # Cron-based file cleanup
├── cli/
│   └── drop.sh             # CLI script
├── templates/              # Jinja2 templates (Pico CSS + HTMX)
├── static/                 # CSS
├── scripts/
│   └── publish.sh          # Docker multi-platform publish
├── Dockerfile
├── docker-compose.yml      # Dev (local volume bind)
├── docker-compose.prod.yml # Prod (pre-built image)
└── project.json            # Publish metadata
```
