package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
	sio "github.com/tomruk/socket.io-go"
)

const (
	addr     = "127.0.0.1:3000"
	certFile = "cert.pem"
	keyFile  = "key.pem"
)

func main() {
	var (
		config   = sio.ServerConfig{}
		server   *http.Server
		wtServer *webtransport.Server
	)

	// If both certificate and key files exist, that means we're going to use TLS.
	// Generate a self-signed SSL certificate with:
	// openssl req -new -x509 -nodes -out cert.pem -keyout key.pem -days 720

	_, errCertFile := os.Stat(certFile)
	_, errKeyFile := os.Stat(keyFile)
	useTLS := !os.IsNotExist(errCertFile) && !os.IsNotExist(errKeyFile)
	if useTLS {
		// If TLS is enabled, use WebTransport.
		wtServer = &webtransport.Server{
			H3: http3.Server{Addr: addr},
		}
		config.EIO.WebTransportServer = wtServer
	}

	io := sio.NewServer(&config)

	api := newAPI()
	api.setup(io.Of("/"))

	err := io.Run()
	if err != nil {
		log.Fatalln(err)
	}

	fs := http.FileServer(http.Dir("public"))
	router := http.NewServeMux()

	if allowOrigin == "" {
		// Make sure to have a slash at the end of the URL.
		// Otherwise instead of matching with this handler, requests might match with a file that has an socket.io prefix (such as socket.io.min.js).
		router.Handle("/socket.io/", io)
	} else {
		if !strings.HasPrefix(allowOrigin, "http://") && !strings.HasPrefix(allowOrigin, "https://") {
			if useTLS {
				allowOrigin = "https://" + allowOrigin
			} else {
				allowOrigin = "http://" + allowOrigin
			}
		}

		fmt.Printf("ALLOW_ORIGIN is set to: %s\n", allowOrigin)
		h := corsMiddleware(io, allowOrigin)

		// Make sure to have a slash at the end of the URL.
		// Otherwise instead of matching with this handler, requests might match with a file that has an socket.io prefix (such as socket.io.min.js).
		router.Handle("/socket.io/", h)
	}
	router.Handle("/", fs)

	server = &http.Server{
		Addr:    addr,
		Handler: router,

		// It is always a good practice to set timeouts.
		ReadTimeout: 120 * time.Second,
		IdleTimeout: 120 * time.Second,

		// HTTPWriteTimeout returns io.PollTimeout + 10 seconds (extra 10 seconds to write the response).
		// You should either set this timeout to 0 (infinite) or some value greater than the io.PollTimeout.
		// Otherwise poll requests may fail.
		WriteTimeout: io.HTTPWriteTimeout(),
	}

	fmt.Printf("Listening on: %s\n", addr)
	if useTLS {
		wtServer.H3.Handler = io
		go wtServer.ListenAndServeTLS(certFile, keyFile)
		err = server.ListenAndServeTLS(certFile, keyFile)
		if err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	} else {
		err = server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}
}
