FROM golang:1.25-alpine AS builder

WORKDIR /src

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime 保留 BusyBox wget，供容器编排执行 HTTP healthcheck。
FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 browseragent \
    && adduser -S -D -H -u 10001 -G browseragent browseragent \
    && mkdir -p /var/lib/browser-agent/artifacts \
    && chown -R browseragent:browseragent /var/lib/browser-agent
COPY --from=builder --chown=browseragent:browseragent /out/api /api
USER browseragent:browseragent
ENV API_ADDR=:8001
EXPOSE 8001
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:8001/healthz || exit 1
CMD ["/api"]
