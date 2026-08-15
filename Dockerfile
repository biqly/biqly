# Multi-binary build: SERVICE selects which directory under cmd/ to compile.
# Defaults to "api"; pass --build-arg SERVICE=migrate (or worker) to build the others.
FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine AS builder

RUN apk --no-cache add ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETARCH
ARG SERVICE=api
RUN test -d "./cmd/${SERVICE}" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /biqly-service "./cmd/${SERVICE}"

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /biqly-service /biqly-service

EXPOSE 8888

USER 65534:65534

ENTRYPOINT ["/biqly-service"]
