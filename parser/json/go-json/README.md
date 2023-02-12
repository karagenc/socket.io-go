# go-json

This package is for [go-json](https://github.com/goccy/go-json) support. go-json is configured via `EncodeOptionFunc` and `DecodeOptionFunc` types.

## Usage

```go
import (
    sio "github.com/tomruk/socket.io-go"
    jsonparser "github.com/tomruk/socket.io-go/parser/json"
    gojson "github.com/tomruk/socket.io-go/parser/json/go-json"
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