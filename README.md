# Socket.IO - Go

This is a [Socket.IO](https://socket.io) library for Go.

# Compatibility

In the near future I plan to support last 2 versions of Go. For now supported Go version is 1.16.

The least supported Socket.IO version is 3.0. If you have an older version of Socket.IO in your other projects, please consider upgrading your dependencies.

| JavaScript Socket.IO version | Socket.IO protocol revision | Engine.IO protocol revision | socket.io-go version |
| ---------------------------- | --------------------------- | --------------------------- | -------------------- |
| 0.9.x                        | 1, 2                        | 1, 2                        | Not supported        |
| 1.x, 2.x                     | 3, 4                        | 3                           | Not supported        |
| 3.x, 4.x                     | 5                           | 4                           | 0.x                  |

## Design Goals

# Q&A

### Why use Socket.IO?

### I want to send big files.

### Sometimes Socket.IO doesn't notice when the network connection is down.

Try reducing pingInterval and pingTimeout.

# Guide

## Transports

If you are a contributor please see: [Developing a Transport](CONTRIBUTING.md#developing-a-transport)

## Reserved events

When you call `socket.On` the handler...

These are reserved events:

```
connect
connect_error
disconnect
disconnecting

open
error
ping

close
reconnect
reconnect_attempt
reconnect_error
reconnect_failed
```

## JSON

JSON serialization is highly customizable. Under the `parser/json/serializer` directory there are different packages for JSON serialization:

| Name                                        | Description                                               | Usage                                      |
| ------------------------------------------- | --------------------------------------------------------- | ------------------------------------------ |
| `stdjson`                                   | stdlib's `encoding/json`. This is the default serializer. | [README.md](parser/json/serializer/stdjson/README.md) |
| [go-json](https://github.com/goccy/go-json) |                                                           | [README.md](parser/json/serializer/go-json/README.md) |
| [sonic](https://github.com/bytedance/sonic) |                                                           | [README.md](parser/json/serializer/sonic/README.md)   |
| `fast`                                      | Conditionally uses sonic or go-json.                      | [README.md](parser/json/serializer/fast/README.md)    |

# Contributing

Contributions are very welcome. Please read [this page](CONTRIBUTING.md).
