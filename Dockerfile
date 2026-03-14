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

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
ENV XDG_CACHE_HOME=/tmp/.cache
COPY --from=build /out/dialtone-watcher /app/dialtone-watcher
ENTRYPOINT ["/app/dialtone-watcher"]
CMD ["help"]
