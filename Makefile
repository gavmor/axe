.PHONY: test lint check

test:
	go test ./...

lint:
	golangci-lint run

check: lint test
