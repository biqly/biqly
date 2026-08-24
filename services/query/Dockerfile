FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine AS builder

RUN apk --no-cache add ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /biqly-query ./services/query/cmd

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /biqly-query /biqly-query

EXPOSE 8888

USER 65534:65534

ENTRYPOINT ["/biqly-query"]
