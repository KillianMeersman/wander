all: test benchmark

test:
	go test ./...

benchmark:
	go test -bench . -benchtime 10s

.PHONY: test benchmark