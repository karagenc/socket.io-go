all: test

test:
	go test -tags deadlock -count 1 -buildmode=default -race -cover -covermode=atomic ./...

build-examples:
	cd _examples && go build ./...
	cd engine.io/_examples && go build ./...

.PHONY: test build-examples
