FROM node:20 AS frontend-builder
WORKDIR /frontend
COPY frontend .
RUN npm install && npm run build

FROM golang:latest AS go-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /frontend/dist ./frontend/dist
RUN go build -o app .

FROM ubuntu as builder
RUN apt-get update && apt-get install -y file tree ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /root
COPY --from=go-builder /app/app .
# COPY --from=go-builder /app/.env .
CMD ["./app"]