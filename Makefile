.PHONY: generate check-generate test lint coverage setup

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

run-frontend:
	cd frontend/tpm-ui && npm run dev

run-backend:
	go run .

run-all: run-backend run-frontend
	cd frontend/tpm-ui && npm run dev

lint:
	golangci-lint run ./...

coverage:
	go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

setup:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go mod download
	@echo "Setup complete. Ensure 'protoc' and 'golangci-lint' are installed on your system."
