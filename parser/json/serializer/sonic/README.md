# sonic

This package is for sonic support. sonic is configured via its `Config` struct. For config and its options, have a look at [api.go](https://github.com/bytedance/sonic/blob/main/api.go).

One thing to keep in mind is that sonic has `EscapeHTML` disabled by default. This might become very dangerous. I recommend you to enable it — you might have a frontend code which processes socket.io input and things like XSS might happen.

In the below code, you can see the recommended configuration options. I explained each option and why it's enabled or disabled.

## Usage

```go
import (
    sio "github.com/karagenc/socket.io-go"
    jsonparser "github.com/karagenc/socket.io-go/parser/json"
    "github.com/karagenc/socket.io-go/parser/json/serializer/sonic"
)

func main() {
    io := sio.NewServer(&sio.ServerConfig{
        ParserCreator: jsonparser.NewCreator(0, sonic.New(sonic.Config{
            // The string is very likely to be used in someplace else.
            // This package is going to be used on a backend, and on the backend variables
            // can be stored for a long time. This means the string and its JSON buffer
            // might not be deallocated for a long time. On the backend, we never want to
            // excessively and redundantly consume memory. So, this is enabled.
            CopyString: true,
            // HTTP response needs to be kept as little as possible.
            // In this context, it is better to sacrifice CPU speed for network speed/latency.
            CompactMarshaler: true,
            // Security. Definitely enable this.
            EscapeHTML: true,
            // Redundant, and the last thing needed on this Earth.
            SortMapKeys: false,
        })),
    })

    io.Run()
}
```
