# Contributing

## Concurrency

## Pre-commit Checklist

- Run all tests.
- Make sure to use `JSONSerializer` inside socket.io parser (`parser/json` package). Do not accidentally use `encoding/json` or other JSON packages.

And you're good to go.

## Developing a Transport

Please make sure to read [transport.go](engine.io/transport.go) carefully.
