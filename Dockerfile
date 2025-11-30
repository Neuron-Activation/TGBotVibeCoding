# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY *.go ./

# Собираем бинарник
RUN go build -o bot .

# Stage 2: Run
FROM alpine:latest

WORKDIR /root/

# Устанавливаем сертификаты (для запросов к API Telegram)
RUN apk --no-cache add ca-certificates

# Копируем бинарник из стадии сборки
COPY --from=builder /app/bot .

# Создаем пустой файл для данных, чтобы избежать проблем с правами (опционально)
RUN touch data.json

# Команда запуска
CMD ["./bot"]