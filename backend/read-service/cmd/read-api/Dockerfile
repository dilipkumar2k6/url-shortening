FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy shared go.mod and go.sum
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy shared code
COPY backend/shared/ shared/

# Copy read-service code
COPY backend/read-service/ read-service/

# Build the binary
RUN go build -o read-api read-service/cmd/read-api/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/read-api .

EXPOSE 8080

CMD ["./read-api"]
