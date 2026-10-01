.PHONY: fmt fmt-check test race lint fix deps check

fmt:
	go fmt ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "run make fmt" && exit 1)

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run ./...

fix:
	golangci-lint run ./... --fix

deps:
	go mod tidy

# What a PR must pass.
check: fmt-check race lint
