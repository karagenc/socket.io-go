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
}

type (
	NamespaceConnectFunc func(socket ServerSocket)
)

func (n *Namespace) OnConnect(f NamespaceConnectFunc) {
	n.connectHandlers.On(&f)
}

func (n *Namespace) OnceConnect(f NamespaceConnectFunc) {
	n.connectHandlers.Once(&f)
}

func (n *Namespace) OffConnect(_f ...NamespaceConnectFunc) {
	f := make([]*NamespaceConnectFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	n.connectHandlers.Off(f...)
}
