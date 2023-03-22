package adapter

type TestSocket struct {
	id SocketID

	Rooms     []Room
	Connected bool
}

func NewTestSocket(id SocketID) *TestSocket {
	return &TestSocket{
		id:        id,
		Connected: true,
		Rooms:     []Room{Room(id)},
	}
}

func (s *TestSocket) ID() SocketID { return s.id }

func (s *TestSocket) Join(room ...Room) {
	s.Rooms = append(s.Rooms, room...)
}

func (s *TestSocket) Leave(room Room) {
	remove := func(slice []Room, s int) []Room {
		return append(slice[:s], slice[s+1:]...)
	}
	for i, r := range s.Rooms {
		if r == room {
			s.Rooms = remove(s.Rooms, i)
		}
	}
}

func (s *TestSocket) Emit(eventName string, v ...any) {}

func (s *TestSocket) To(room ...Room) *BroadcastOperator { return nil }

func (s *TestSocket) In(room ...Room) *BroadcastOperator { return nil }

func (s *TestSocket) Except(room ...Room) *BroadcastOperator { return nil }

func (s *TestSocket) Broadcast() *BroadcastOperator { return nil }

func (s *TestSocket) Disconnect(close bool) {
	s.Connected = false
}
