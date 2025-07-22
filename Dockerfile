FROM --platform=${BUILDPLATFORM:-linux/amd64} ghcr.io/castai/live/golang:1.24.4-alpine AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

ENV GOCACHE=/go-cache
ENV GOMODCACHE=/gomod-cache

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN --mount=type=cache,target=/go-cache \
    go build -o /app -ldflags="-s -w" ./app/

FROM gcr.io/distroless/static-debian12:latest

COPY --from=builder --chown=nonroot:nonroot /app /app

USER nonroot:nonroot
ENTRYPOINT ["/app"]
