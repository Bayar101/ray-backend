# ———— Build stage ————
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server

# ———— Runtime stage ————
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /src/server .
COPY config.yml .
EXPOSE 8080
CMD ["./server"]
