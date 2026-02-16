FROM python:3.12-slim

WORKDIR /app

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

# Environment variables
ENV PYTHONUNBUFFERED=1
ENV DATA_DIR=/data
ENV PYTHONPATH=/app/backend

EXPOSE 8802

CMD ["uvicorn", "backend.main:app", "--host", "0.0.0.0", "--port", "8802"]
