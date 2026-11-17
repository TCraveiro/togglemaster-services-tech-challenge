# --- Estágio 1: Build (instala dependências) ---
FROM python:3.11-slim-bookworm AS builder

WORKDIR /app
COPY requirements.txt .
# Depois das dependências da aplicação, o opentelemetry-bootstrap detecta as bibliotecas
# instaladas (Flask, requests, psycopg2, botocore...) e gera a lista de instrumentações
RUN pip install --no-cache-dir --user --upgrade pip setuptools wheel && \
    pip install --no-cache-dir --user -r requirements.txt && \
    PATH=/root/.local/bin:$PATH opentelemetry-bootstrap -a requirements > otel-requirements.txt && \
    pip install --no-cache-dir --user -r otel-requirements.txt

# --- Estágio 2: Runtime ---
FROM python:3.11-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir --upgrade pip setuptools wheel --break-system-packages

COPY --from=builder /root/.local /root/.local
RUN find / -path "*/pip/_vendor/bom.cdx.json" -delete 2>/dev/null || true
COPY . .

ENV PATH=/root/.local/bin:$PATH \
    PYTHONUNBUFFERED=1

EXPOSE 8003

# opentelemetry-instrument ativa a instrumentação automática sem alterar o código da aplicação
CMD ["opentelemetry-instrument", "gunicorn", "--bind", "0.0.0.0:8003", "app:app"]
