FROM golang:1.26.0-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o /bin/server ./cmd/server/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /bin/server /app/server

ENV PORT=8080

EXPOSE 8080

CMD ["/app/server"]