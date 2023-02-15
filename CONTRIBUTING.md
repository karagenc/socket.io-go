# Contributing

## Concurrency

## Pre-commit Checklist

- Run all tests.
- Make sure to use `JSONSerializer` inside socket.io parser (`parser/json/serializer` package). Do not accidentally use `encoding/json` or other JSON packages.
- Make sure to use Mutex on:
    - `Client.eio`
- Make sure to use `InternalError` for socket.io internal errors.
    - Use `wrapInternalError` when an error is this package's responsibility.
    - Internal server error vs bad request error

And you're good to go.

## Developing a Transport

Please make sure to read [transport.go](engine.io/transport.go) carefully.
