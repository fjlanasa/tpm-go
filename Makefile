.PHONY: generate test

generate:
	protoc --go_out=. --go_opt=paths=source_relative api/v1/events/transit.proto

test: generate
	go test -v ./... 