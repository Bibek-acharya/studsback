FROM golang:1.26.0-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -o /bin/server ./cmd/server/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates chromium font-noto fontconfig

WORKDIR /app

COPY --from=builder /bin/server /app/server

ENV PORT=8080

EXPOSE 8080

CMD ["/app/server"]