# Contributing

## Concurrency

Socket.IO is a highly concurrent library. One has to be very careful with concurrency and synchronization.

## Pre-commit Checklist

- Run all tests.
- Make sure to use `JSONSerializer` inside socket.io parser (`parser/json/serializer` package). Do not accidentally use `encoding/json` or other JSON packages.
- Make sure to use mutexes correctly.
    - Beware of struct fields that require a mutex.
- Make sure to use `InternalError` for socket.io internal errors.
    - Use `wrapInternalError` when an error is this package's responsibility.
    - Internal server error vs bad request error
- Make sure to use exported types on the `Adapter` interface.
    - One exception is `ackHandler`; it doesn't need to be exported.

And you're good to go.

## Debugging

Add to VSCode config.json:
```json
"go.testFlags": [
    "-race",
    "-args",
    "-test.v"
  ],
"go.testTags": "deadlock",
```

## Developing a Transport

Please make sure to read [transport.go](engine.io/transport.go) carefully.
