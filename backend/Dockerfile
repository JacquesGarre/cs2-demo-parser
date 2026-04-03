# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/api

FROM alpine:3.21
WORKDIR /app
RUN adduser -D appuser
RUN mkdir -p /app/uploads && chmod 0777 /app/uploads
USER appuser

COPY --from=builder /out/server /app/server
EXPOSE 8080
ENV PORT=8080
ENV UPLOADS_DIR=/app/uploads

CMD ["/app/server"]
