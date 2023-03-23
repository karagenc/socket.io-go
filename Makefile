all: test

.PHONY: test
test:
	go test -count 1 -buildmode=default -race -cover -covermode=atomic ./...

.PHONY: build-examples
build-examples:
	cd _examples && go build ./...
	cd engine.io/_examples && go build ./...
