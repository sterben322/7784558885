FROM golang:1.23-alpine AS builder

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary for Linux container runtime
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -trimpath -ldflags="-s -w" -o /app/main .

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/main ./main
COPY --from=builder /app/web ./web

EXPOSE 8080
CMD ["./main"]
