package sio

import (
	"sync"

	"github.com/tomruk/socket.io-go/adapter"
	"github.com/tomruk/socket.io-go/parser"
)

type (
	clientSocketStore struct {
		sockets map[string]*clientSocket
		mu      sync.Mutex
	}

	serverSocketStore struct {
		socketsByID        map[SocketID]*serverSocket
		socketsByNamespace map[string]*serverSocket
		mu                 sync.Mutex
	}

	// We could've used mapset instead of this,
	// but mapset doesn't have an equivalent of the GetOrCreate method.
	namespaceStore struct {
		nsps map[string]*Namespace
		mu   sync.Mutex
	}

	namespaceSocketStore struct {
		sockets map[SocketID]ServerSocket
		mu      sync.Mutex
	}

	// This is to ensure we have a socket store with a
	// right function signature that matches with adapter's
	// `SocketStore`.
	adapterSocketStore struct {
		store *namespaceSocketStore
	}
)

func newClientSocketStore() *clientSocketStore {
	return &clientSocketStore{sockets: make(map[string]*clientSocket)}
}

func newServerSocketStore() *serverSocketStore {
	return &serverSocketStore{
		socketsByID:        make(map[SocketID]*serverSocket),
		socketsByNamespace: make(map[string]*serverSocket),
	}
}

func newNamespaceStore() *namespaceStore {
	return &namespaceStore{nsps: make(map[string]*Namespace)}
}

func newNamespaceSocketStore() *namespaceSocketStore {
	return &namespaceSocketStore{sockets: make(map[SocketID]ServerSocket)}
}

func newAdapterSocketStore(store *namespaceSocketStore) *adapterSocketStore {
	return &adapterSocketStore{store: store}
}

func (s *serverSocketStore) getByID(sid SocketID) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.socketsByID[sid]
	return
}

func (s *serverSocketStore) getByNsp(nsp string) (socket *serverSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok = s.socketsByNamespace[nsp]
	return
}

func (s *serverSocketStore) getAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.socketsByID))
	i := 0
	for _, socket := range s.socketsByID {
		sockets[i] = socket
		i++
	}
	return
}

func (s *serverSocketStore) getAndRemoveAll() (sockets []*serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*serverSocket, len(s.socketsByID))
	i := 0
	for _, socket := range s.socketsByID {
		sockets[i] = socket
		i++
	}
	s.socketsByID = make(map[SocketID]*serverSocket)
	s.socketsByNamespace = make(map[string]*serverSocket)
	return
}

func (s *serverSocketStore) set(socket *serverSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.socketsByID[socket.ID()] = socket
	s.socketsByNamespace[socket.nsp.Name()] = socket
}

func (s *serverSocketStore) removeByID(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	socket, ok := s.socketsByID[sid]
	if ok {
		delete(s.socketsByID, sid)
		delete(s.socketsByNamespace, socket.nsp.Name())
	}
}

func (s *namespaceStore) getOrCreate(
	name string,
	server *Server,
	adapterCreator adapter.Creator,
	parserCreator parser.Creator,
) (namespace *Namespace, created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ok bool
	namespace, ok = s.nsps[name]
	if !ok {
		namespace = newNamespace(name, server, adapterCreator, parserCreator)
		s.nsps[namespace.Name()] = namespace
		created = true
	}
	return
}

func (s *namespaceStore) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.nsps)
}

func (s *namespaceStore) set(nsp *Namespace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nsps[nsp.Name()] = nsp
}

func (s *namespaceStore) get(name string) (nsp *Namespace, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nsp, ok = s.nsps[name]
	return
}

func (s *namespaceStore) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nsps, name)
}

// Send Engine.IO packets to a specific socket.
func (s *namespaceSocketStore) sendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	_socket, ok := s.get(sid)
	if !ok {
		return false
	}
	socket := _socket.(*serverSocket)
	socket.conn.sendBuffers(buffers...)
	return true
}

func (s *namespaceSocketStore) get(sid SocketID) (so ServerSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	so, ok = s.sockets[sid]
	return so, ok
}

func (s *namespaceSocketStore) getAll() []ServerSocket {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets := make([]ServerSocket, len(s.sockets))
	i := 0
	for _, s := range s.sockets {
		sockets[i] = s
		i++
	}
	return sockets
}

func (s *namespaceSocketStore) set(so ServerSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[so.ID()] = so
}

func (s *namespaceSocketStore) remove(sid SocketID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, sid)
}

// Send Engine.IO packets to a specific socket.
func (s *adapterSocketStore) SendBuffers(sid SocketID, buffers [][]byte) (ok bool) {
	return s.store.sendBuffers(sid, buffers)
}

func (s *adapterSocketStore) Get(sid SocketID) (so adapter.Socket, ok bool) {
	return s.store.get(sid)
}

func (s *adapterSocketStore) GetAll() []adapter.Socket {
	_sockets := s.store.getAll()
	sockets := make([]adapter.Socket, len(_sockets))
	for i := range sockets {
		sockets[i] = _sockets[i]
	}
	return sockets
}

func (s *adapterSocketStore) Remove(sid SocketID) {
	s.store.remove(sid)
}

func (s *clientSocketStore) get(namespace string) (ss *clientSocket, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss, ok = s.sockets[namespace]
	return
}

func (s *clientSocketStore) getAll() (sockets []*clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sockets = make([]*clientSocket, len(s.sockets))
	i := 0
	for _, ss := range s.sockets {
		sockets[i] = ss
		i++
	}
	return
}

func (s *clientSocketStore) set(ss *clientSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[ss.namespace] = ss
}

func (s *clientSocketStore) remove(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, namespace)
}
