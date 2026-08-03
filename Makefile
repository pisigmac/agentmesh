.PHONY: proto build test lint docker-up docker-down verify-openapi

export PATH := /usr/local/go/bin:$(HOME)/go/bin:$(PATH)
GOTOOLCHAIN ?= go1.22.5

proto:
	protoc -I api/proto --go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		api/proto/agentmesh/v1/agentmesh.proto

build: proto
	GOTOOLCHAIN=$(GOTOOLCHAIN) go build -o bin/control-plane ./cmd/control-plane
	GOTOOLCHAIN=$(GOTOOLCHAIN) go build -o bin/dataplane ./cmd/dataplane
	GOTOOLCHAIN=$(GOTOOLCHAIN) go build -o bin/agentmesh ./cmd/agentmesh

test:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go test ./...

lint:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go vet ./...

docker-up:
	./scripts/start-all.sh

docker-down:
	./scripts/stop-all.sh

start:
	./scripts/start-all.sh

stop:
	./scripts/stop-all.sh

status:
	./scripts/status-all.sh

verify-openapi:
	test -f docs/openapi.yml
