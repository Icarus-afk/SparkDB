.PHONY: build install run test vet clean dev docker-build help lint

BINARY=sparkdb

build:
	go build -o $(BINARY) ./cmd/sparkdb

install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)

run: build
	./$(BINARY) start

test:
	go test ./...
	./test.sh

vet:
	go vet ./...

lint:
	go vet ./...
	staticcheck ./... 2>/dev/null || true

clean:
	rm -f $(BINARY) sparkdb_system.db* main testdb *.crt *.key
	rm -rf backups/

init: build
	./$(BINARY) init

dev: build
	@echo "=== SparkDB dev server ==="
	@echo "  Web console: http://localhost:9600"
	@echo "  Default auth: admin / admin"
	@echo ""
	./$(BINARY) start

fresh: clean dev

docker-build:
	docker build -t sparkdb .

docker-run:
	docker compose up -d

docker-stop:
	docker compose down

help:
	@echo "Targets:"
	@echo "  build       - Build the sparkdb binary"
	@echo "  install     - Build and install to /usr/local/bin"
	@echo "  init        - Initialize a SparkDB project"
	@echo "  run         - Build and start the server"
	@echo "  dev         - Build and start (with info message)"
	@echo "  fresh       - Clean database files and start fresh"
	@echo "  test        - Run Go tests and integration suite"
	@echo "  vet         - Run go vet"
	@echo "  clean       - Remove binary and database files"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Start Docker Compose services"
	@echo "  docker-stop  - Stop Docker Compose services"
