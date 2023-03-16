package adapter

type testSocket struct {
	id        SocketID
	rooms     []Room
	connected bool
}

func newTestSocketWithID(id SocketID) *testSocket {
	return &testSocket{
		id:        id,
		connected: true,
		rooms:     []Room{Room(id)},
	}
}

func (s *testSocket) ID() SocketID { return s.id }

func (s *testSocket) Join(room ...Room) {
	s.rooms = append(s.rooms, room...)
}

func (s *testSocket) Leave(room Room) {
	remove := func(slice []Room, s int) []Room {
		return append(slice[:s], slice[s+1:]...)
	}
	for i, r := range s.rooms {
		if r == room {
			s.rooms = remove(s.rooms, i)
		}
	}
}

func (s *testSocket) Emit(eventName string, v ...any) {}

func (s *testSocket) To(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) In(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) Except(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) Broadcast() *BroadcastOperator { return nil }

func (s *testSocket) Disconnect(close bool) {
	s.connected = false
}
