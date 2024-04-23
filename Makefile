all: test

test:
	go test -tags deadlock -count 1 -buildmode=default -race -cover -covermode=atomic ./...

build-examples:
	cd examples && go build ./...
	cd engine.io/examples && go build ./...

.PHONY: test build-examples
