.PHONY: build test test-short test-cover bench lint clean docker compose-up compose-down run proto k8s-up k8s-down

BIN_DIR := bin
BINARY := $(BIN_DIR)/order-server

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/order-server

test:
	go test -race -v ./...

test-short:
	go test -race -short ./...

test-cover:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html

bench:
	go test -bench=. -benchmem ./internal/cache/... ./internal/domain/...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BIN_DIR) coverage.txt coverage.html

proto:
	@find gen -name '*.go' -delete 2>/dev/null || true
	protoc \
		--go_out=. --go_opt=module=github.com/glinharesb/order-flow \
		--go-grpc_out=. --go-grpc_opt=module=github.com/glinharesb/order-flow \
		-I proto \
		proto/order/v1/*.proto

docker:
	docker build -f docker/Dockerfile -t order-flow:latest .

compose-up:
	docker compose up -d

compose-down:
	docker compose down -v

run: build
	$(BINARY)

k8s-up:
	kind create cluster --name order-flow 2>/dev/null || true
	docker build -f docker/Dockerfile -t order-flow:latest .
	kind load docker-image order-flow:latest --name order-flow
	kubectl apply -f deploy/k8s/namespace.yaml
	kubectl apply -f deploy/k8s/

k8s-down:
	kind delete cluster --name order-flow
