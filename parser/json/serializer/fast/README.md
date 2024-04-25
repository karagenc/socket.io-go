# Fast JSON Serializer

This is a package that contains code to use the fastest JSON serializer possible. The idea of combining [sonic](https://github.com/bytedance/sonic) and [go-json](https://github.com/goccy/go-json) into one package seemed very gorgeous to me, so I wrote [fj4echo](https://github.com/tomruk/fj4echo). Now I'm applying the same concept to this library.

Selection of JSON library is platform dependent. If the platform supports sonic (at the time of writing sonic supports AMD64 and Linux, macOS, & Windows), then sonic is used, otherwise go-json is used.

Also note that go-json's version is `0.10.0` at the time of writing. Please consider your backend's stability before using it.

## Usage

```go
import (
    sio "github.com/tomruk/socket.io-go"
    jsonparser "github.com/tomruk/socket.io-go/parser/json"
    "github.com/tomruk/socket.io-go/parser/json/serializer/fast"
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
    "github.com/tomruk/socket.io-go/parser/json/serializer/fast"
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
