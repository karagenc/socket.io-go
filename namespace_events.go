package sio

func (n *Namespace) OnEvent(eventName string, handler any) {
	if IsEventReservedForNsp(eventName) {
		panic("sio: OnEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	n.eventHandlers.on(eventName, h)
}

func (n *Namespace) OnceEvent(eventName string, handler any) {
	if IsEventReservedForNsp(eventName) {
		panic("sio: OnceEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	n.eventHandlers.once(eventName, h)
}

func (n *Namespace) OffEvent(eventName string, handler ...any) {
	n.eventHandlers.off(eventName, handler...)
}

func (n *Namespace) OffAll() {
	n.eventHandlers.offAll()
	n.connectionHandlers.offAll()
}

type (
	NamespaceConnectionFunc func(socket ServerSocket)
)

func (n *Namespace) OnConnection(f NamespaceConnectionFunc) {
	n.connectionHandlers.on(&f)
}

func (n *Namespace) OnceConnection(f NamespaceConnectionFunc) {
	n.connectionHandlers.once(&f)
}

func (n *Namespace) OffConnection(_f ...NamespaceConnectionFunc) {
	f := make([]*NamespaceConnectionFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	n.connectionHandlers.off(f...)
}
