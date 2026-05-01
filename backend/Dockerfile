FROM golang:1.25.3-alpine AS build

WORKDIR /app

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-mod=readonly

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app
COPY --from=build /out/server /app/server

USER app:app

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/server"]
