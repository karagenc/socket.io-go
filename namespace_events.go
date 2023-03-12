package sio

func (n *Namespace) OnEvent(eventName string, handler any) {
	if IsEventReservedForNsp(eventName) {
		panic("sio: OnEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	n.eventHandlers.On(eventName, newEventHandler(handler))
}

func (n *Namespace) OnceEvent(eventName string, handler any) {
	if IsEventReservedForNsp(eventName) {
		panic("sio: OnceEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	n.eventHandlers.Once(eventName, newEventHandler(handler))
}

func (n *Namespace) OffEvent(eventName string, handler ...any) {
	n.eventHandlers.Off(eventName, handler...)
}

func (n *Namespace) OffAll() {
	n.eventHandlers.OffAll()
	n.connectionHandlers.OffAll()
}

type (
	NamespaceConnectionFunc   func(socket ServerSocket)
	NamespaceNewNamespaceFunc func(namespace *Namespace)
)

func (n *Namespace) OnConnection(f NamespaceConnectionFunc) {
	n.connectionHandlers.On(&f)
}

func (n *Namespace) OnceConnection(f NamespaceConnectionFunc) {
	n.connectionHandlers.Once(&f)
}

func (n *Namespace) OffConnection(_f ...NamespaceConnectionFunc) {
	f := make([]*NamespaceConnectionFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	n.connectionHandlers.Off(f...)
}

func (n *Namespace) OnNewNamespace(f NamespaceNewNamespaceFunc) {
	if n.name != "/" {
		panic("sio: OnNewNamespace is for `/` namespace only")
	}
	n.newNamespaceHandlers.On(&f)
}

func (n *Namespace) OnceNewNamespace(f NamespaceNewNamespaceFunc) {
	if n.name != "/" {
		panic("sio: OnNewNamespace is for `/` namespace only")
	}
	n.newNamespaceHandlers.Once(&f)
}

func (n *Namespace) OffNewNamespace(_f ...NamespaceNewNamespaceFunc) {
	if n.name != "/" {
		panic("sio: OnNewNamespace is for `/` namespace only")
	}
	f := make([]*NamespaceNewNamespaceFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	n.newNamespaceHandlers.Off(f...)
}
