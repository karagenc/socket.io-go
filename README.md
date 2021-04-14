# Socket.IO - Go

This is a [Socket.IO](https://socket.io) library for Go.

# Compatibility

This library supports Socket.IO version 3.0 or later.

| JavaScript Socket.IO version | Socket.IO protocol revision | Engine.IO protocol revision | socket.io-go version
| ----------- | ----------- | ---------- | ------------- |
| 0.9.x       | 1, 2        | 1, 2       | Not supported |
| 1.x, 2.x    | 3, 4        | 3          | Not supported |
| 3.x, 4.x    | 5           | 4          | 0.x |

## Design Goals

# Q & A

### Why use Socket.IO?

### I want to send big files.

### Sometimes Socket.IO doesn't notice when the network connection is down

Try reducing pingInterval and pingTimeout.

# Guide

## Transports

If you are contributing to this library please see: [Writing a New Transport.](CONTRIBUTING.md#writing-a-new-transport)

## JSON

You can use [jsoniter](https://github.com/json-iterator/go) for JSON encoding instead of encoding/json.

```shell
go build -tags=jsoniter .
```

This is adopted from [gin](https://github.com/gin-gonic/gin).

# Contributing

Contributions are very welcome. Please read [this page](CONTRIBUTING.md).
