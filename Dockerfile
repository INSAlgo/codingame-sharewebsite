FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/static ./static

RUN adduser -D -u 1000 appuser && \
	mkdir -p .cert-cache && \
	chown -R appuser:appuser /app

USER appuser

EXPOSE 80 443 8080

CMD ["./server"]
