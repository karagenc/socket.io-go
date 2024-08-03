package sio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	eio "github.com/karagenc/socket.io-go/engine.io"
	"github.com/karagenc/socket.io-go/internal/sync"
	"github.com/karagenc/socket.io-go/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"
)

func TestServer(t *testing.T) {
	t.Run("should receive events", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("random", func(a int, b string, c []int) {
				assert.Equal(t, 1, a)
				assert.Equal(t, "2", b)
				assert.Equal(t, []int{3}, c)
				tw.Done()
			})
		})
		socket.Emit("random", 1, "2", []int{3})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should error with null messages", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("message", func(a any) {
				assert.Equal(t, nil, a)
				tw.Done()
			})
		})
		socket.Emit("message", nil)
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		socket.OnEvent("woot", func(a string) {
			assert.Equal(t, "tobi", a)
			tw.Done()
		})
		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", "tobi")
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with utf8 multibyte character", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(3)

		socket.OnEvent("hoot", func(a string) {
			assert.Equal(t, "utf8 — string", a)
			tw.Done()
		})
		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("hoot", "utf8 — string")
			socket.Emit("hoot", "utf8 — string")
			socket.Emit("hoot", "utf8 — string")
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with binary data", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		socket.OnEvent("randomBin", func(a Binary) {
			assert.Equal(t, randomBin, []byte(a))
			tw.Done()
		})
		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("randomBin", Binary(randomBin))
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with binary data", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("randomBin", func(a Binary) {
				assert.Equal(t, randomBin, []byte(a))
				tw.Done()
			})
		})
		socket.Connect()
		socket.Emit("randomBin", Binary(randomBin))

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with several types of data (including binary)", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("multiple", func(a int, b string, c []int, d Binary, e []any) {
				assert.Equal(t, 1, a)
				assert.Equal(t, "3", b)
				assert.Equal(t, []int{4}, c)
				assert.Equal(t, randomBin, []byte(d))
				assert.Len(t, e, 2)
				assert.Equal(t, float64(5), e[0])
				assert.Equal(t, "swag", e[1])
				tw.Done()
			})
		})
		socket.Connect()
		socket.Emit("multiple", 1, "3", []int{4}, Binary(randomBin), []any{5, "swag"})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive all events emitted from namespaced client immediately and in order", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/chat", nil)
		tw := utils.NewTestWaiter(2)

		total := 0
		countMu := sync.Mutex{}

		io.Of("/chat").OnConnection(func(socket ServerSocket) {
			socket.OnEvent("hi", func(letter string) {
				countMu.Lock()
				defer countMu.Unlock()
				total++
				switch total {
				case 1:
					assert.Equal(t, 'a', int32(letter[0]))
				case 2:
					assert.Equal(t, 'b', int32(letter[0]))
				}
				tw.Done()
			})
		})
		socket.Connect()
		socket.Emit("hi", "a")
		socket.Emit("hi", "b")

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive event with callbacks", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(fn func(int, int)) {
				fn(1, 2)
			})
		})
		socket.Connect()
		socket.Emit("woot", func(a, b int) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with callbacks", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(fn func(int, int)) {
				fn(1, 2)
			})
		})
		socket.Connect()
		socket.Emit("woot", func(a, b int) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with args and callback", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(a, b int, c func()) {
				assert.Equal(t, 1, a)
				assert.Equal(t, 2, b)
				c()
			})
		})
		socket.Connect()
		socket.Emit("woot", 1, 2, func() {
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with args and callback", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(2)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", 1, 2, func() {
				tw.Done()
			})
		})
		socket.OnEvent("woot", func(a, b int, c func()) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			c()
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events with binary args and callbacks", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(a Binary, b func(int, int)) {
				assert.Equal(t, randomBin, []byte(a))
				b(1, 2)
			})
		})
		socket.Connect()
		socket.Emit("woot", Binary(randomBin), func(a, b int) {
			assert.Equal(t, 1, a)
			assert.Equal(t, 2, b)
			tw.Done()
		})

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with binary args and callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", Binary(randomBin), func(a, b int) {
				assert.Equal(t, 1, a)
				assert.Equal(t, 2, b)
				tw.Done()
			})
		})
		socket.OnEvent("woot", func(a Binary, b func(int, int)) {
			assert.Equal(t, randomBin, []byte(a))
			b(1, 2)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events with binary args and callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("woot", Binary(randomBin), func(a, b int) {
				assert.Equal(t, 1, a)
				assert.Equal(t, 2, b)
				tw.Done()
			})
		})
		socket.OnEvent("woot", func(a Binary, b func(int, int)) {
			assert.Equal(t, randomBin, []byte(a))
			b(1, 2)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should emit events and receive binary data in a callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.Emit("hi", func(a Binary) {
				assert.Equal(t, Binary(randomBin), a)
				tw.Done()
			})
		})
		socket.OnEvent("hi", func(ack func(Binary)) {
			ack(randomBin)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should receive events and pass binary data in a callback", func(t *testing.T) {
		randomBin := []byte("\x36\x43\x78\x6a\x4c\xad\x7b\x6f\x33\x96\xc6\xdb\x4b\xd3\xe4\x8c\xc7\x12")

		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		socket := manager.Socket("/", nil)
		tw := utils.NewTestWaiter(1)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("woot", func(ack func(Binary)) {
				ack(randomBin)
			})
		})
		socket.Emit("woot", func(a Binary) {
			assert.Equal(t, Binary(randomBin), a)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should be able to emit after server close and restart", func(t *testing.T) {
		var (
			reconnectionDelay    = 100 * time.Millisecond
			reconnectionDelayMax = 100 * time.Millisecond
		)
		io, ts, manager, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				EIO: eio.ServerConfig{
					PingTimeout:  3000 * time.Millisecond,
					PingInterval: 1000 * time.Millisecond,
				},
			},
			&ManagerConfig{
				ReconnectionDelay:    &reconnectionDelay,
				ReconnectionDelayMax: &reconnectionDelayMax,
				EIO: eio.ClientConfig{
					Transports: []string{"polling"}, // To buy time by not waiting for +2 other transport's connection attempts.
				},
			},
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.OnEvent("ev", func(data string) {
				assert.Equal(t, "payload", data)
				tw.Done()
			})
		})

		socket.OnceConnect(func() {
			manager.OnReconnect(func(attempt uint32) {
				socket.Emit("ev", "payload")
			})

			go func() {
				ts.Close()
				time.Sleep(5000 * time.Millisecond)
				hs := http.Server{
					Addr:    ts.Listener.Addr().String(),
					Handler: io,
				}
				err := hs.ListenAndServe()
				if err != nil && err != http.ErrServerClosed {
					panic(err)
				}
			}()
		})
		socket.Connect()

		tw.WaitTimeout(t, 20*time.Second)
		close()
	})

	t.Run("should leave all rooms joined after a middleware failure", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			socket.Join("room1")
			return fmt.Errorf("nope")
		})
		socket.OnConnectError(func(err any) {
			_, ok := io.Of("/").Adapter().SocketRooms(socket.ID())
			assert.False(t, ok)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should leave all rooms joined after a middleware failure", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.Disconnect(true)
			socket.Join("room1")
		})
		socket.OnDisconnect(func(reason Reason) {
			_, ok := io.Of("/").Adapter().SocketRooms(socket.ID())
			assert.False(t, ok)
			tw.Done()
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should timeout if the client does not acknowledge the event", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.Timeout(100*time.Millisecond).Emit("unknown", func(err error) {
				assert.NotNil(t, err)
				tw.Done()
			})
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should timeout if the client does not acknowledge the event in time", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.Timeout(1*time.Nanosecond).Emit("echo", 42, func(err error, n int) {
				assert.NotNil(t, err)
				tw.Done()
			})
		})
		socket.OnEvent("echo", func(n int, ack func(int)) {
			ack(n)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		time.Sleep(200 * time.Millisecond)
		close()
	})

	t.Run("should not timeout if the client does acknowledge the event", func(t *testing.T) {
		io, _, manager, close := newTestServerAndClient(
			t,
			nil,
			nil,
		)
		tw := utils.NewTestWaiter(1)
		socket := manager.Socket("/", nil)

		io.OnConnection(func(socket ServerSocket) {
			socket.Timeout(1*time.Second).Emit("echo", 42, func(err error, n int) {
				assert.Nil(t, err)
				assert.Equal(t, 42, n)
				tw.Done()
			})
		})
		socket.OnEvent("echo", func(n int, ack func(int)) {
			ack(n)
		})
		socket.Connect()

		tw.WaitTimeout(t, utils.DefaultTestWaitTimeout)
		close()
	})

	t.Run("should close the connection when receiving several CONNECT packets", func(t *testing.T) {
		_, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)

		sid := utils.EIOHandshake(t, ts)
		// Send the first CONNECT packet
		utils.EIOPush(t, ts, sid, "40")
		// Send another CONNECT packet
		utils.EIOPush(t, ts, sid, "40")
		// Wait for socket to close.
		time.Sleep(500 * time.Millisecond)
		// Session is cleanly closed (not discarded)
		body, code := utils.EIOPoll(t, ts, sid)
		assert.Equal(t, http.StatusBadRequest, code)
		m := make(map[string]interface{})
		err := json.Unmarshal([]byte(body), &m)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, eio.ErrorUnknownSID, int(m["code"].(float64)))
		serverError, ok := eio.GetServerError(eio.ErrorUnknownSID)
		assert.True(t, ok)
		assert.Equal(t, serverError.Message, m["message"])

		close()
	})

	t.Run("should close the connection when receiving an EVENT packet while not connected", func(t *testing.T) {
		_, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)

		sid := utils.EIOHandshake(t, ts)
		// Send an EVENT packet.
		utils.EIOPush(t, ts, sid, `42["some event"]`)
		// Wait for socket to close.
		time.Sleep(500 * time.Millisecond)
		// Session is cleanly closed.
		body, code := utils.EIOPoll(t, ts, sid)
		assert.Equal(t, http.StatusBadRequest, code)
		m := make(map[string]interface{})
		err := json.Unmarshal([]byte(body), &m)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, eio.ErrorUnknownSID, int(m["code"].(float64)))
		serverError, ok := eio.GetServerError(eio.ErrorUnknownSID)
		assert.True(t, ok)
		assert.Equal(t, serverError.Message, m["message"])

		close()
	})

	t.Run("should close the connection when receiving an invalid packet", func(t *testing.T) {
		_, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)

		sid := utils.EIOHandshake(t, ts)
		// Send a CONNECT packet.
		utils.EIOPush(t, ts, sid, "40")
		// Send an invalid packet.
		utils.EIOPush(t, ts, sid, "4abc")
		// Wait for socket to close.
		time.Sleep(500 * time.Millisecond)
		// Session is cleanly closed (not discarded, see 'client.close()')
		// First, we receive the Socket.IO handshake response
		body, code := utils.EIOPoll(t, ts, sid)
		assert.Equal(t, http.StatusBadRequest, code)
		m := make(map[string]interface{})
		err := json.Unmarshal([]byte(body), &m)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, eio.ErrorUnknownSID, int(m["code"].(float64)))
		serverError, ok := eio.GetServerError(eio.ErrorUnknownSID)
		assert.True(t, ok)
		assert.Equal(t, serverError.Message, m["message"])

		close()
	})

	restoreSessionInit := func(t *testing.T, io *Server, ts *httptest.Server) (sioSid, sioPid, offset string) {
		// Engine.IO handshake
		sid := utils.EIOHandshake(t, ts)

		// Socket.IO handshake
		utils.EIOPush(t, ts, sid, "40")
		handshakeBody, status := utils.EIOPoll(t, ts, sid)
		assert.Equal(t, http.StatusOK, status)
		if !strings.HasPrefix(handshakeBody, "40") {
			t.FailNow()
		}
		handshakeBody = handshakeBody[2:]
		m := make(map[string]string)
		err := json.Unmarshal([]byte(handshakeBody), &m)
		if err != nil {
			t.Fatal(err)
		}
		var ok bool
		sioSid, ok = m["sid"]
		require.True(t, ok)
		// In that case, the handshake also contains a private session ID.
		sioPid, ok = m["pid"]
		require.True(t, ok)

		io.Emit("hello")

		message, status := utils.EIOPoll(t, ts, sid)
		require.Equal(t, http.StatusOK, status)
		if !strings.HasPrefix(message, `42["hello"`) {
			t.FailNow()
		}

		message = message[2:]
		var messageSlice []string
		err = json.Unmarshal([]byte(message), &messageSlice)
		if err != nil {
			t.Fatal(err)
		}
		require.Len(t, messageSlice, 2)
		offset = messageSlice[1]

		utils.EIOPush(t, ts, sid, "1")
		return
	}

	t.Run("should restore session and missed packets", func(t *testing.T) {
		io, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				ServerConnectionStateRecovery: ServerConnectionStateRecovery{
					Enabled: true,
				},
			},
			nil,
		)
		ts.Client().Timeout = 1000 * time.Millisecond
		var (
			serverSocket ServerSocket
			mu           sync.Mutex
		)
		io.OnceConnection(func(socket ServerSocket) {
			socket.Join("room1")
			mu.Lock()
			serverSocket = socket
			mu.Unlock()
		})

		sioSid, sioPid, offset := restoreSessionInit(t, io, ts)

		io.Emit("hello1")             // Broadcast
		io.To("room1").Emit("hello2") // Broadcast to room
		mu.Lock()
		serverSocket.Emit("hello3") // Direct message
		mu.Unlock()

		newSid := utils.EIOHandshake(t, ts)
		utils.EIOPush(t, ts, newSid, fmt.Sprintf(`40{"pid":"%s","offset":"%s"}`, sioPid, offset))

		packets := []string{}
		for i := 0; i < 4; i++ {
			payload, status := utils.EIOPoll(t, ts, newSid)
			assert.Equal(t, http.StatusOK, status)
			packets = append(packets, strings.Split(payload, "\x1e")...)
			if len(packets) == 4 {
				break
			}
		}
		assert.Len(t, packets, 4)

		if !strings.HasPrefix(packets[0], `42["hello1"`) {
			t.FailNow()
		}
		if !strings.HasPrefix(packets[1], `42["hello2"`) {
			t.FailNow()
		}
		if !strings.HasPrefix(packets[2], `42["hello3"`) {
			t.FailNow()
		}
		if !strings.HasPrefix(packets[3], fmt.Sprintf(`40{"sid":"%s","pid":"%s"}`, sioSid, sioPid)) {
			t.FailNow()
		}

		close()
	})

	t.Run("should restore rooms and data attributes", func(t *testing.T) {
		io, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				ServerConnectionStateRecovery: ServerConnectionStateRecovery{
					Enabled: true,
				},
			},
			nil,
		)
		ts.Client().Timeout = 1000 * time.Millisecond

		io.OnceConnection(func(socket ServerSocket) {
			assert.False(t, socket.Recovered())
			socket.Join("room1")
			socket.Join("room2")
		})

		sioSid, sioPid, offset := restoreSessionInit(t, io, ts)

		socketChan := make(chan ServerSocket)
		io.OnceConnection(func(socket ServerSocket) {
			socketChan <- socket
		})

		newSid := utils.EIOHandshake(t, ts)
		utils.EIOPush(t, ts, newSid, fmt.Sprintf(`40{"pid":"%s","offset":"%s"}`, sioPid, offset))

		socket := <-socketChan
		assert.Equal(t, SocketID(sioSid), socket.ID())
		assert.True(t, socket.Recovered())
		assert.True(t, socket.Rooms().Contains("room1", "room2"))

		close()
	})

	t.Run("should not run middlewares upon recovery by default", func(t *testing.T) {
		io, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				ServerConnectionStateRecovery: ServerConnectionStateRecovery{
					Enabled: true,
				},
			},
			nil,
		)
		ts.Client().Timeout = 1000 * time.Millisecond

		// Ensure middleware normally gets to run.
		ok := false
		setOkToTrue := sync.OnceFunc(func() {
			ok = true
		})
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			setOkToTrue()
			return nil
		})

		sioSid, sioPid, offset := restoreSessionInit(t, io, ts)

		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			t.Fatal("should not happen")
			return nil
		})

		socketChan := make(chan ServerSocket)
		io.OnceConnection(func(socket ServerSocket) {
			socketChan <- socket
		})

		newSid := utils.EIOHandshake(t, ts)
		utils.EIOPush(t, ts, newSid, fmt.Sprintf(`40{"pid":"%s","offset":"%s"}`, sioPid, offset))

		socket := <-socketChan
		assert.Equal(t, SocketID(sioSid), socket.ID())
		assert.True(t, socket.Recovered())

		assert.True(t, ok)

		close()
	})

	t.Run("should run middlewares even upon recovery", func(t *testing.T) {
		io, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				ServerConnectionStateRecovery: ServerConnectionStateRecovery{
					Enabled:        true,
					UseMiddlewares: true,
				},
			},
			nil,
		)
		ts.Client().Timeout = 1000 * time.Millisecond

		sioSid, sioPid, offset := restoreSessionInit(t, io, ts)

		ok := false
		setOkToTrue := sync.OnceFunc(func() {
			ok = true
		})
		io.Use(func(socket ServerSocket, handshake *Handshake) any {
			setOkToTrue()
			return nil
		})

		socketChan := make(chan ServerSocket)
		io.OnceConnection(func(socket ServerSocket) {
			socketChan <- socket
		})

		newSid := utils.EIOHandshake(t, ts)
		utils.EIOPush(t, ts, newSid, fmt.Sprintf(`40{"pid":"%s","offset":"%s"}`, sioPid, offset))

		socket := <-socketChan
		assert.Equal(t, SocketID(sioSid), socket.ID())
		assert.True(t, socket.Recovered())

		assert.True(t, ok)

		close()
	})

	t.Run("should fail to restore an unknown session", func(t *testing.T) {
		_, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
				ServerConnectionStateRecovery: ServerConnectionStateRecovery{
					Enabled: true,
				},
			},
			nil,
		)
		ts.Client().Timeout = 1000 * time.Millisecond

		newSid := utils.EIOHandshake(t, ts)
		utils.EIOPush(t, ts, newSid, `40{"pid":"foo","offset":"bar"}`)

		packet, status := utils.EIOPoll(t, ts, newSid)
		assert.Equal(t, http.StatusOK, status)

		if !strings.HasPrefix(packet, "40") {
			t.FailNow()
		}
		assert.NotContains(t, packet, "foo")
		assert.NotContains(t, packet, "bar")

		close()
	})

	t.Run("should be disabled by default", func(t *testing.T) {
		_, ts, _, close := newTestServerAndClient(
			t,
			&ServerConfig{
				AcceptAnyNamespace: true,
			},
			nil,
		)
		ts.Client().Timeout = 1000 * time.Millisecond

		newSid := utils.EIOHandshake(t, ts)
		utils.EIOPush(t, ts, newSid, "40")

		handshakeBody, status := utils.EIOPoll(t, ts, newSid)
		assert.Equal(t, http.StatusOK, status)
		if !strings.HasPrefix(handshakeBody, "40") {
			t.FailNow()
		}
		handshakeBody = handshakeBody[2:]
		m := make(map[string]string)
		err := json.Unmarshal([]byte(handshakeBody), &m)
		if err != nil {
			t.Fatal(err)
		}
		var ok bool
		sioSid, ok := m["sid"]
		require.True(t, ok)
		require.NotEmpty(t, sioSid)
		_, ok = m["pid"]
		require.False(t, ok)

		close()
	})
}

func newTestServerAndClient(
	t *testing.T,
	serverConfig *ServerConfig,
	managerConfig *ManagerConfig,
) (
	io *Server,
	ts *httptest.Server,
	manager *Manager,
	close func(),
) {
	enablePrintDebugger := os.Getenv("SIO_DEBUGGER_PRINT") == "1"
	enablePrintDebuggerEIO := os.Getenv("EIO_DEBUGGER_PRINT") == "1"

	if serverConfig == nil {
		serverConfig = new(ServerConfig)
	}
	if enablePrintDebugger {
		serverConfig.Debugger = NewPrintDebugger()
	}
	if enablePrintDebuggerEIO {
		serverConfig.EIO.Debugger = NewPrintDebugger()
	}
	serverConfig.EIO.WebSocketAcceptOptions = &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	}

	io = NewServer(serverConfig)
	err := io.Run()
	if err != nil {
		t.Fatal(err)
	}

	ts = httptest.NewServer(io)
	manager = NewManager(ts.URL, managerConfig)

	return io, ts, manager, func() {
		err = io.Close()
		if err != nil {
			t.Fatalf("io.Close: %s", err)
		}
		ts.Close()
	}
}

func newTestManager(ts *httptest.Server, managerConfig *ManagerConfig) *Manager {
	enablePrintDebugger := os.Getenv("SIO_DEBUGGER_PRINT") == "1"
	enablePrintDebuggerEIO := os.Getenv("EIO_DEBUGGER_PRINT") == "1"
	if managerConfig == nil {
		managerConfig = new(ManagerConfig)
	}
	if enablePrintDebugger {
		managerConfig.Debugger = NewPrintDebugger()
	}
	if enablePrintDebuggerEIO {
		managerConfig.EIO.Debugger = NewPrintDebugger()
	}
	managerConfig.EIO.WebSocketDialOptions = &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	}
	return NewManager(ts.URL, managerConfig)
}
