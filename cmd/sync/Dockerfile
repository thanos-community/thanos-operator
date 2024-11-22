# syntax=docker/dockerfile:1

FROM golang:1.24 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY cmd/sync/main.go .
COPY internal ./internal
COPY api ./api

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o config-syncer main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/config-syncer .
USER 65532:65532

ENTRYPOINT ["/config-syncer"]
