all: test benchmark

test:
	go test ./...

benchmark:
	go test -bench .

.PHONY: test benchmark