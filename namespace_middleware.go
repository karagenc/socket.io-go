package sio

type NspMiddlewareFunc func(socket ServerSocket, handshake *Handshake) error

func (n *Namespace) Use(f NspMiddlewareFunc) {
	n.middlewareFuncsMu.Lock()
	defer n.middlewareFuncsMu.Unlock()
	n.middlewareFuncs = append(n.middlewareFuncs, f)
}

func (n *Namespace) runMiddlewares(socket *serverSocket, handshake *Handshake) error {
	n.middlewareFuncsMu.RLock()
	defer n.middlewareFuncsMu.RUnlock()

	for _, f := range n.middlewareFuncs {
		err := f(socket, handshake)
		if err != nil {
			return err
		}
	}
	return nil
}
