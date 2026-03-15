FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy shared go.mod and go.sum
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy shared code
COPY backend/shared/ shared/

# Copy write-service code
COPY backend/write-service/ write-service/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o cdc-worker write-service/cmd/cdc-worker/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/cdc-worker .

CMD ["./cdc-worker"]
