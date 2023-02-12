# Fast JSON Serializer

This is a package that contains code to use the fastest JSON serializer possible. The idea of combining sonic and go-json into one package seemed very gorgeous to me, so I wrote [fj4echo](https://github.com/tomruk/fj4echo). Now I'm applying the same concept to my socket.io library.

## Usage

```go
import (
    sio "github.com/tomruk/socket.io-go"
    jsonparser "github.com/tomruk/socket.io-go/parser/json"
    "github.com/tomruk/socket.io-go/parser/json/fast"
)

func main() {
    io := sio.NewServer(&sio.ServerConfig{
        ParserCreator: jsonparser.NewCreator(0, fast.New()),
    })

    io.Run()
}
```

You can programmatically check which serializer is being used with SerializerType enum (see [serializer_type.go](serializer_type.go)):

```go
serializerType := fast.Type()
switch serializerType {
    case fast.SerializerTypeSonic:
        fmt.Println("sonic is choosen. This means that the processor is amd64")
    case fast.SerializerTypeGoJSON:
        fmt.Println("go-json is choosen")
}

```

## Customization

```go
import (
    sio "github.com/tomruk/socket.io-go"
    jsonparser "github.com/tomruk/socket.io-go/parser/json"
    "github.com/tomruk/socket.io-go/parser/json/fast"
    "github.com/bytedance/sonic"
    "github.com/goccy/go-json"
)

func main() {
    io := sio.NewServer(&sio.ServerConfig{
        ParserCreator: jsonparser.NewCreator(0, fast.NewWithConfig(fast.Config{
            SonicConfig: sonic.Config{
                CopyString:  true,
                SortMapKeys: false,
            },

            GoJSON: fast.GoJSONConfig{
                EncodeOptions: []json.EncodeOptionFunc{
                    json.UnorderedMap(),
                },
                DecodeOptions: []json.DecodeOptionFunc{
                    json.DecodeFieldPriorityFirstWin(),
                },
            },
        })),
    })

    io.Run()
}
```