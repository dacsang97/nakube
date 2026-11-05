.PHONY: test vet lint fmt-check snapshot

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run --timeout=5m

fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

snapshot:
	goreleaser build --clean --snapshot
