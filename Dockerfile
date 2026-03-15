# syntax=docker/dockerfile:1.7

FROM golang:1.25-bookworm AS base
WORKDIR /src
ENV CGO_ENABLED=0

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	go mod download

COPY . .

FROM base AS test
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	GOOS=linux GOARCH=amd64 go test ./...

FROM base AS build
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/dialtone-watcher .

FROM debian:bookworm-slim AS runtime
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
	bash \
	ca-certificates \
	curl \
	dnsutils \
	iproute2 \
	procps \
	&& rm -rf /var/lib/apt/lists/* \
	&& useradd --create-home --home-dir /home/app --shell /bin/bash app
ENV XDG_CACHE_HOME=/tmp/.cache
COPY --from=build /out/dialtone-watcher /app/dialtone-watcher
RUN chown -R app:app /app /home/app
USER app
ENTRYPOINT ["/app/dialtone-watcher"]
CMD ["help"]
