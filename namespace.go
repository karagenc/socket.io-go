package sio

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/tomruk/socket.io-go/adapter"
	"github.com/tomruk/socket.io-go/parser"
)

type Namespace struct {
	name   string
	server *Server

	debug Debugger

	sockets *NamespaceSocketStore

	middlewareFuncs   []NspMiddlewareFunc
	middlewareFuncsMu sync.RWMutex

	adapter adapter.Adapter
	parser  parser.Parser

	ackID uint64
	ackMu sync.Mutex

	eventHandlers      *eventHandlerStore
	connectionHandlers *handlerStore[*NamespaceConnectionFunc]
}

func newNamespace(name string, server *Server, adapterCreator adapter.Creator, parserCreator parser.Creator) *Namespace {
	socketStore := newNamespaceSocketStore()
	nsp := &Namespace{
		name:               name,
		server:             server,
		debug:              server.debug.WithContext("Namespace with name: " + name),
		sockets:            socketStore,
		parser:             parserCreator(),
		eventHandlers:      newEventHandlerStore(),
		connectionHandlers: newHandlerStore[*NamespaceConnectionFunc](),
	}
	nsp.adapter = adapterCreator(newAdapterSocketStore(socketStore), parserCreator)
	return nsp
}

func (n *Namespace) Name() string { return n.name }

func (n *Namespace) Adapter() adapter.Adapter { return n.adapter }

// Emits an event to all connected clients in the given namespace.
func (n *Namespace) Emit(eventName string, v ...any) {
	adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).Emit(eventName, v...)
}

// Sends a message to the other Socket.IO servers of the cluster.
func (n *Namespace) ServerSideEmit(eventName string, _v ...any) {
	header := &parser.PacketHeader{
		Type:      parser.PacketTypeEvent,
		Namespace: n.Name(),
	}

	if IsEventReservedForNsp(eventName) {
		panic("sio: broadcastOperator.Emit: attempted to emit to a reserved event")
	}

	// One extra space for eventName,
	// the other for ID (see the Broadcast method of sessionAwareAdapter)
	v := make([]any, 0, len(_v)+1)
	v = append(v, eventName)
	v = append(v, _v...)

	n.adapter.ServerSideEmit(header, v)
}

func (n *Namespace) OnServerSideEmit(eventName string, _v ...any) {
	values := make([]reflect.Value, len(_v))
	for i, v := range _v {
		values[i] = reflect.ValueOf(v)
	}
	handlers := n.eventHandlers.GetAll(eventName)

	go func() {
		for _, handler := range handlers {
			if len(values) == len(handler.inputArgs) {
				for i, v := range values {
					if handler.inputArgs[i].Kind() != reflect.Ptr && v.Kind() == reflect.Ptr {
						values[i] = v.Elem()
					}
				}
			} else {
				n.debug.Log("Namespace.OnServerSideEmit: handler signature mismatch")
				return
			}
			handler.Call(values...)
		}
	}()
}

// Sets a modifier for a subsequent event emission that the event
// will only be broadcast to clients that have joined the given room.
//
// To emit to multiple rooms, you can call `To` several times.
func (n *Namespace) To(room ...Room) *BroadcastOperator {
	return adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).To(room...)
}

// Alias of To(...)
func (n *Namespace) In(room ...Room) *BroadcastOperator {
	return adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).In(room...)
}

// Sets a modifier for a subsequent event emission that the event
// will only be broadcast to clients that have not joined the given rooms.
func (n *Namespace) Except(room ...Room) *BroadcastOperator {
	return adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).Except(room...)
}

// Compression flag is unused at the moment, thus setting this will have no effect on compression.
func (n *Namespace) Compress(compress bool) *BroadcastOperator {
	return adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).Compress(compress)
}

// Sets a modifier for a subsequent event emission that the event data will only be broadcast to the current node (when scaling to multiple nodes).
//
// See: https://socket.io/docs/v4/using-multiple-nodes
func (n *Namespace) Local() *BroadcastOperator {
	return adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).Local()
}

// Gets the sockets of the namespace.
// Beware that this is local to the current node. For sockets across all nodes, use FetchSockets
func (n *Namespace) Sockets() []ServerSocket {
	return n.sockets.GetAll()
}

// Returns the matching socket instances. This method works across a cluster of several Socket.IO servers.
func (n *Namespace) FetchSockets() []adapter.Socket {
	return adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).FetchSockets()
}

// Makes the matching socket instances join the specified rooms.
func (n *Namespace) SocketsJoin(room ...Room) {
	adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).SocketsJoin(room...)
}

// Makes the matching socket instances leave the specified rooms.
func (n *Namespace) SocketsLeave(room ...Room) {
	adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).SocketsLeave(room...)
}

// Makes the matching socket instances disconnect from the namespace.
//
// If value of close is true, closes the underlying connection. Otherwise, it just disconnects the namespace.
func (n *Namespace) DisconnectSockets(close bool) {
	adapter.NewBroadcastOperator(n.Name(), n.adapter, n.parser, IsEventReservedForServer).DisconnectSockets(close)
}

type authRecoveryFields struct {
	SessionID string
	Offset    string
}

func (n *Namespace) add(c *serverConn, auth json.RawMessage) (*serverSocket, error) {
	n.debug.Log("Adding a new socket to namespace", n.name)

	var (
		handshake = &Handshake{
			Time: time.Now(),
			Auth: auth,
		}
		authRecoveryFields authRecoveryFields
		socket             *serverSocket
	)

	err := json.Unmarshal(auth, &authRecoveryFields)
	if err != nil {
		return nil, err
	}

	if n.server.connectionStateRecovery.Enabled {
		session, ok := n.adapter.RestoreSession(adapter.PrivateSessionID(authRecoveryFields.SessionID), authRecoveryFields.Offset)
		if ok {
			socket, err = newServerSocket(n.server, c, n, c.parser, session)
			if err != nil {
				return nil, err
			}
		} else {
			n.debug.Log("`session` is nil")
		}
	}

	// If connection state recovery is disabled
	// or for some reason socket couldn't be retrieved
	if socket == nil {
		socket, err = newServerSocket(n.server, c, n, c.parser, nil)
		if err != nil {
			return nil, err
		}
	}

	if n.server.connectionStateRecovery.Enabled && !n.server.connectionStateRecovery.UseMiddlewares && socket.Recovered() {
		return socket, n.doConnect(socket)
	}

	err = n.runMiddlewares(socket, handshake)
	if err != nil {
		return nil, err
	}

	return socket, n.doConnect(socket)
}

func (n *Namespace) doConnect(socket *serverSocket) error {
	n.sockets.Set(socket)

	// It is paramount that the internal `onconnect` logic
	// fires before user-set events to prevent state order
	// violations (such as a disconnection before the connection
	// logic is complete)
	err := socket.onConnect()
	if err != nil {
		return err
	}

	go func() {
		for _, handler := range n.connectionHandlers.GetAll() {
			(*handler)(socket)
		}
	}()
	return nil
}

func (n *Namespace) remove(socket *serverSocket) {
	if _, ok := n.sockets.Get(socket.ID()); ok {
		n.sockets.Remove(socket.ID())
	} else {
		n.debug.Log("Ignoring remove for", socket.ID())
	}
}

func (n *Namespace) nextAckID() uint64 {
	n.ackMu.Lock()
	defer n.ackMu.Unlock()
	id := n.ackID
	n.ackID++
	return id
}
