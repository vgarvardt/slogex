.PHONY: lint
lint:
	golangci-lint run --config=./.github/linters/.golangci.yml --fix

.PHONY: test
test:
	go test -timeout=2m -cover -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: spellcheck
spellcheck:
	docker run \
		--interactive --tty --rm \
		--volume "$(CURDIR):/workdir" \
		--workdir "/workdir" \
		python:3.12-slim bash -c "python -m pip install --upgrade pip && pip install 'codespell>=2.2.4' && codespell"
