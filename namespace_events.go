package sio

func (n *Namespace) OnEvent(eventName string, handler any) {
	if IsEventReservedForNsp(eventName) {
		panic("sio: OnEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	n.eventHandlers.On(eventName, h)
}

func (n *Namespace) OnceEvent(eventName string, handler any) {
	if IsEventReservedForNsp(eventName) {
		panic("sio: OnceEvent: attempted to register a reserved event: `" + eventName + "`")
	}
	h, err := newEventHandler(handler)
	if err != nil {
		panic(err)
	}
	n.eventHandlers.Once(eventName, h)
}

func (n *Namespace) OffEvent(eventName string, handler ...any) {
	n.eventHandlers.Off(eventName, handler...)
}

func (n *Namespace) OffAll() {
	n.eventHandlers.OffAll()
	n.connectionHandlers.OffAll()
}

type (
	NamespaceConnectionFunc func(socket ServerSocket)
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
