FROM golang:alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /sparkdb ./cmd/sparkdb

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S sparkdb && \
    adduser -S sparkdb -G sparkdb -h /data && \
    mkdir -p /data /backups /etc/sparkdb && \
    chown -R sparkdb:sparkdb /data /backups /etc/sparkdb
COPY --from=builder /sparkdb /sparkdb
USER sparkdb
EXPOSE 9600
VOLUME ["/data", "/backups", "/etc/sparkdb"]
ENTRYPOINT ["/sparkdb"]
CMD ["start"]
