package adapter

type testSocket struct {
	id SocketID

	Rooms     []Room
	Connected bool
}

var _ Socket = newTestSocket("")

func newTestSocket(id SocketID) *testSocket {
	return &testSocket{
		id:        id,
		Connected: true,
		Rooms:     []Room{Room(id)},
	}
}

func (s *testSocket) ID() SocketID { return s.id }

func (s *testSocket) Join(room ...Room) {
	s.Rooms = append(s.Rooms, room...)
}

func (s *testSocket) Leave(room Room) {
	remove := func(slice []Room, s int) []Room {
		return append(slice[:s], slice[s+1:]...)
	}
	for i, r := range s.Rooms {
		if r == room {
			s.Rooms = remove(s.Rooms, i)
		}
	}
}

func (s *testSocket) Emit(eventName string, v ...any) {}

func (s *testSocket) To(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) In(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) Except(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) Broadcast() *BroadcastOperator { return nil }

func (s *testSocket) Disconnect(close bool) {
	s.Connected = false
}
