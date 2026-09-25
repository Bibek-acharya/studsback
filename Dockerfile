FROM golang:1.26.0-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev git

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -o /bin/server ./cmd/server

FROM alpine:3.20

# ffmpeg/ffprobe are required at RUNTIME: uploaded video lectures are
# normalized to a broadly playable H.264/AAC MP4 on upload. Without ffmpeg the
# video upload path answers 503 instead of storing unplayable bytes.
RUN apk add --no-cache ca-certificates chromium font-noto fontconfig ffmpeg

WORKDIR /app

COPY --from=builder /bin/server /app/server

ENV PORT=8080

EXPOSE 8080

CMD ["/app/server"]