# go-json

This package is for [go-json](https://github.com/goccy/go-json) support. go-json is configured via `EncodeOptionFunc` and `DecodeOptionFunc` types.

Note that go-json's version is `0.10.0` at the time of writing. Please consider your backend's stability before using it.

## Usage

```go
import (
    sio "github.com/karagenc/socket.io-go"
    jsonparser "github.com/karagenc/socket.io-go/parser/json"
    gojson "github.com/karagenc/socket.io-go/parser/json/serializer/go-json"
    "github.com/goccy/go-json"
)

func main() {
    io := sio.NewServer(&sio.ServerConfig{
        ParserCreator: jsonparser.NewCreator(0, gojson.New(
            []json.EncodeOptionFunc{
                json.UnorderedMap(),
            },
            []json.DecodeOptionFunc{
                json.DecodeFieldPriorityFirstWin(),
            },
        )),
    })

    io.Run()
}
```
