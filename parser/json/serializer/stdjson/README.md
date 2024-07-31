# stdjson

This package is for `encoding/json` support. `encoding/json` has no configuration.

## Usage

```go
import (
    sio "github.com/karagenc/socket.io-go"
    jsonparser "github.com/karagenc/socket.io-go/parser/json"
    "github.com/karagenc/socket.io-go/parser/json/serializer/stdjson"
)

func main() {
    io := sio.NewServer(&sio.ServerConfig{
        ParserCreator: jsonparser.NewCreator(0, stdjson.New()),
    })

    io.Run()
}
```
