FROM python:3.12-slim

WORKDIR /app

# Install cron
RUN apt-get update && apt-get install -y --no-install-recommends cron && \
    rm -rf /var/lib/apt/lists/*

# Copy dependency file and install
COPY pyproject.toml ./
RUN pip install --no-cache-dir . && \
    rm -rf /root/.cache/pip && \
    find /usr/local -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true

# Copy application
COPY backend/ ./backend/
COPY templates/ ./templates/
COPY static/ ./static/

# Create data directory
RUN mkdir -p /data

# Setup cron — cleanup every 12 hours
RUN echo "0 */12 * * * cd /app && PYTHONPATH=/app/backend DATA_DIR=/data python backend/cleanup.py >> /var/log/cron.log 2>&1" > /etc/cron.d/drop-cleanup && \
    chmod 0644 /etc/cron.d/drop-cleanup && \
    crontab /etc/cron.d/drop-cleanup

# Environment variables
ENV PYTHONUNBUFFERED=1
ENV DATA_DIR=/data
ENV PYTHONPATH=/app/backend

EXPOSE 8802

# Start cron in background, then uvicorn
CMD cron && uvicorn backend.main:app --host 0.0.0.0 --port 8802
