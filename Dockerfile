FROM golang:alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /sparkdb ./cmd/sparkdb

FROM scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /sparkdb /sparkdb
EXPOSE 9600
VOLUME ["/data", "/backups", "/etc/sparkdb"]
ENTRYPOINT ["/sparkdb"]
CMD ["start"]
