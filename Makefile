.PHONY: air test test-integration build vet

air:
	air

vet:
	go vet ./...

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

build:
	go build ./...