package sio

type NamespaceNewNamespaceFunc func(namespace *Namespace)

func (s *Server) OnNewNamespace(f NamespaceNewNamespaceFunc) {
	s.newNamespaceHandlers.On(&f)
}

func (s *Server) OnceNewNamespace(f NamespaceNewNamespaceFunc) {
	s.newNamespaceHandlers.Once(&f)
}

func (s *Server) OffNewNamespace(_f ...NamespaceNewNamespaceFunc) {
	f := make([]*NamespaceNewNamespaceFunc, len(_f))
	for i := range f {
		f[i] = &_f[i]
	}
	s.newNamespaceHandlers.Off(f...)
}
