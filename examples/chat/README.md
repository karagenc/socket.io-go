# Chat Example

## To use WebTransport

This example supports WebTransport. To use it:

```shell
cd server
# Generate self signed certificates
openssl req -new -x509 -nodes -out cert.pem -keyout key.pem -days 720
go run .
```

On another shell:

```shell
cd client
# Notice that "https" is used as the URL scheme.
go run . -u william -c "https://127.0.0.1:3000/socket.io"
```
