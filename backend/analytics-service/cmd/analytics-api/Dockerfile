FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy shared go.mod and go.sum
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy shared code
COPY backend/shared/ shared/

# Copy analytics-service code
COPY backend/analytics-service/ analytics-service/

# Build the binary
RUN go build -o analytics-api analytics-service/cmd/analytics-api/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/analytics-api .

EXPOSE 8080

CMD ["./analytics-api"]
