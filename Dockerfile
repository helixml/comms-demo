# mock-channels — multi-stage build.
# CGO is required for github.com/mattn/go-sqlite3.

FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ENV CGO_ENABLED=1

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-s -w" \
        -o /out/mock-channels ./cmd/mock-channels

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        wget \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 1000 mockchan \
    && useradd -u 1000 -g mockchan -s /bin/sh -m mockchan \
    && mkdir -p /data && chown mockchan:mockchan /data

COPY --from=builder /out/mock-channels /usr/local/bin/mock-channels

ENV MOCK_CHANNELS_ADDR=:7765 \
    MOCK_CHANNELS_DB=/data/mock-channels.db

USER mockchan
WORKDIR /data
VOLUME ["/data"]

EXPOSE 7765

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:7765/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/mock-channels"]
CMD ["serve"]
