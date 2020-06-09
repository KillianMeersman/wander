all: test benchmark

test:
	go test ./...

benchmark:
	go test -bench . -benchmem ./...

.PHONY: test benchmark