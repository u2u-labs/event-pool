# === Builder Stage ===
FROM golang:1.23-alpine AS builder

RUN apk update && apk add --no-cache build-base cmake git

WORKDIR /app

# Copy only necessary files for go mod
COPY go.mod go.sum ./
COPY prisma ./prisma/
COPY pkg ./pkg/
COPY internal ./internal/

# Download dependencies and generate Prisma client
RUN go mod download
RUN go install github.com/steebchen/prisma-client-go@latest
RUN go run github.com/steebchen/prisma-client-go generate

# Copy rest of the source
COPY . .

# Tidy up and build
RUN go mod tidy
RUN go build -ldflags="-w -s" -o event-pool

# === Final Stage ===
FROM alpine:latest

# Install only required runtime tools
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary and entrypoint
COPY --from=builder /app/event-pool /app/event-pool
COPY --from=builder /app/docker-entrypoint.sh /app/docker-entrypoint.sh
COPY --from=builder /app/prisma /app/prisma
COPY --from=builder /app/pkg /app/pkg
COPY --from=builder /app/data /app/data

RUN chmod +x /app/docker-entrypoint.sh

ENTRYPOINT ["/app/docker-entrypoint.sh"]
