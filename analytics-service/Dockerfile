# --- Estágio 1: Build (instala dependências) ---
FROM python:3.11-slim AS builder

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# --- Estágio 2: Runtime ---
FROM python:3.11-slim

WORKDIR /app

COPY --from=builder /root/.local /root/.local
COPY . .

ENV PATH=/root/.local/bin:$PATH \
    PYTHONUNBUFFERED=1

EXPOSE 8005

# Só 1 worker: o worker SQS roda em thread de background dentro do processo.
# Com mais de 1 worker gunicorn, teríamos múltiplos consumidores concorrentes
# competindo pela mesma fila (o que até funciona, mas não é o comportamento
# pretendido pelo desafio para o teste local).
CMD ["gunicorn", "--bind", "0.0.0.0:8005", "--workers", "1", "app:app"]
