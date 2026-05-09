FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /biqly ./cmd/api/

FROM alpine:3.19

RUN apk --no-cache add ca-certificates \
    && addgroup -S biqly \
    && adduser -S biqly -G biqly

WORKDIR /home/biqly

COPY --from=builder /biqly .
RUN chown biqly:biqly /home/biqly/biqly

USER biqly

EXPOSE 8888

CMD ["./biqly"]
