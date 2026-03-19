.PHONY: generate check-generate test lint coverage setup check-deps run

# Tool versions
PROTOC_GEN_GO_VERSION := v1.35.2

check-deps:
	@missing=0; \
	if ! command -v protoc >/dev/null 2>&1; then \
		echo "✗ protoc not found"; \
		echo "  Install: brew install protobuf"; \
		missing=1; \
	else \
		echo "✓ protoc found: $$(protoc --version)"; \
	fi; \
	if ! command -v protoc-gen-go >/dev/null 2>&1; then \
		echo "✗ protoc-gen-go not found"; \
		echo "  Install: go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)"; \
		missing=1; \
	else \
		echo "✓ protoc-gen-go found"; \
	fi; \
	if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "✗ golangci-lint not found"; \
		echo "  Install: brew install golangci-lint"; \
		missing=1; \
	else \
		echo "✓ golangci-lint found: $$(golangci-lint --version 2>&1 | head -1)"; \
	fi; \
	if [ $$missing -ne 0 ]; then \
		echo ""; \
		echo "Install all missing dependencies and re-run 'make setup'."; \
		exit 1; \
	fi

generate:
	protoc \
		--proto_path=. \
		--proto_path=third_party \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go_opt=Mthird_party/gtfs/gtfs-realtime.proto=github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs \
		api/v1/events/transit.proto

check-generate: generate
	@if ! git diff --quiet -- '*.pb.go'; then \
		echo "ERROR: Generated protobuf files are out of date. Run 'make generate' and commit the results."; \
		git diff -- '*.pb.go'; \
		exit 1; \
	fi

generate-openapi:
	protoc --openapi_out=./api/v1/events/ api/v1/events/transit.proto

test:
	go test -v ./...

run:
	go run .

lint:
	golangci-lint run ./...

coverage:
	go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

setup: check-deps
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go mod download
	@echo "Setup complete. All dependencies verified."
