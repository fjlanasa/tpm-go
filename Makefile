.PHONY: generate test

generate:
	protoc --go_out=. --go_opt=paths=source_relative api/v1/events/transit.proto

generate-openapi:
	protoc --openapi_out=./api/v1/events/ api/v1/events/transit.proto

test: generate
	go test -v ./... 

run: generate
	go run .

run-frontend:
	cd frontend/tpm-ui && npm run dev

run-backend:
	go run .

run-all: run-backend run-frontend
	cd frontend/tpm-ui && npm run dev
