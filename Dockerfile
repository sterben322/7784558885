FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN go build -mod=mod -o main .

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/main ./main
COPY --from=builder /app/web ./web

EXPOSE 8080
CMD ["./main"]
