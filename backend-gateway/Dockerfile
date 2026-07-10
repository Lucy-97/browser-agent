FROM golang:1.22-alpine AS builder

WORKDIR /src

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/gateway ./cmd/gateway

# Stage 2: Minimal runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/gateway /gateway
EXPOSE 8080
CMD ["/gateway"]
