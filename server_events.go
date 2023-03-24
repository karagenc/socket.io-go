package sio

type (
	ServerNewNamespaceFunc  func(namespace *Namespace)
	ServerAnyConnectionFunc func(namespace string, socket ServerSocket)
)

func (s *Server) OnNewNamespace(f ServerNewNamespaceFunc) {
	s.newNamespaceHandlers.on(&f)
}

func (s *Server) OnceNewNamespace(f ServerNewNamespaceFunc) {
	s.newNamespaceHandlers.once(&f)
}

func (s *Server) OffNewNamespace(_f ...ServerNewNamespaceFunc) {
	f := make([]*ServerNewNamespaceFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.newNamespaceHandlers.off(f...)
}

func (s *Server) OnAnyConnection(f ServerAnyConnectionFunc) {
	s.anyConnectionHandlers.on(&f)
}

func (s *Server) OnceAnyConnection(f ServerAnyConnectionFunc) {
	s.anyConnectionHandlers.once(&f)
}

func (s *Server) OffAnyConnection(_f ...ServerAnyConnectionFunc) {
	f := make([]*ServerAnyConnectionFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.anyConnectionHandlers.off(f...)
}
