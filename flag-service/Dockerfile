# --- Estágio 1: Build (instala dependências) ---
FROM python:3.11-slim AS builder

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# --- Estágio 2: Runtime ---
FROM python:3.11-slim

WORKDIR /app

# Copia as libs instaladas no estágio anterior
COPY --from=builder /root/.local /root/.local
COPY . .

ENV PATH=/root/.local/bin:$PATH \
    PYTHONUNBUFFERED=1

EXPOSE 8002

CMD ["gunicorn", "--bind", "0.0.0.0:8002", "app:app"]
