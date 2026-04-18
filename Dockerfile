# ── Stage 1: builder ───────────────────────────────────────────────────────────
FROM golang:1.26-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o helixtrace-api .


# ── Stage 2: runtime ───────────────────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

ENV TZ=Etc/UTC \
    LANG=en_US.UTF-8 \
    LANGUAGE=en_US:en \
    LC_ALL=en_US.UTF-8

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/helixtrace-api .

EXPOSE 8000

CMD ["./helixtrace-api"]
