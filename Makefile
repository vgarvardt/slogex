# Lint
.PHONY: lint
lint:
	golangci-lint run --config=./.github/linters/.golangci.yml --fix

# Test
.PHONY: test
test:
	go test -timeout=2m -cover -coverprofile=coverage.txt -covermode=atomic ./...
