FROM golang:1.25.3-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o server ./cmd/server

FROM alpine:3.22

WORKDIR /app
COPY --from=build /app/server /app/server

EXPOSE 8080
CMD ["/app/server"]
