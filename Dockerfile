# ============================
# Стадия сборки
# ============================
FROM golang:1.22-bookworm AS builder

# Нужно для sqlite (cgo)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Сначала зависимости (для кеша)
COPY go.mod go.sum ./
RUN go mod download

# Потом остальной код
COPY . .

# Сборка бинарника
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o aurora-bot ./cmd/bot

# ============================
# Рантайм
# ============================
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем только бинарник и нужные ресурсы
COPY --from=builder /app/aurora-bot /app/aurora-bot
COPY lore /app/lore
COPY migrations /app/migrations

# Папка для sqlite БД
RUN mkdir -p /app/data

ENV DB_PATH=/app/data/aurora.db

CMD ["/app/aurora-bot"]
