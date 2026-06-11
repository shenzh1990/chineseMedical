FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20

RUN adduser -D -H appuser
USER appuser

WORKDIR /app
COPY --from=builder /out/server /app/server

EXPOSE 8080
ENTRYPOINT ["/app/server"]
