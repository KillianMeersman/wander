all: test benchmark

test:
	go test ./...

benchmark:
	go test -bench . -benchtime 5s

.PHONY: test benchmark