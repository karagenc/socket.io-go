package sio

type NamespaceNewNamespaceFunc func(namespace *Namespace)

func (s *Server) OnNewNamespace(f NamespaceNewNamespaceFunc) {
	s.newNamespaceHandlers.on(&f)
}

func (s *Server) OnceNewNamespace(f NamespaceNewNamespaceFunc) {
	s.newNamespaceHandlers.once(&f)
}

func (s *Server) OffNewNamespace(_f ...NamespaceNewNamespaceFunc) {
	f := make([]*NamespaceNewNamespaceFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.newNamespaceHandlers.off(f...)
}
