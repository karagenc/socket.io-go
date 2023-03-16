package adapter

import eio "github.com/tomruk/socket.io-go/engine.io"

type testSocket struct {
	id SocketID
}

func newTestSocket() *testSocket {
	id, err := eio.GenerateBase64ID(eio.Base64IDSize)
	if err != nil {
		panic(err)
	}
	return &testSocket{
		id: SocketID(id),
	}
}

func newTestSocketWithID(id SocketID) *testSocket {
	return &testSocket{
		id: id,
	}
}

func (s *testSocket) ID() SocketID { return s.id }

func (s *testSocket) Join(room ...Room) {}

func (s *testSocket) Leave(room Room) {}

func (s *testSocket) Emit(eventName string, v ...any) {}

func (s *testSocket) To(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) In(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) Except(room ...Room) *BroadcastOperator { return nil }

func (s *testSocket) Broadcast() *BroadcastOperator { return nil }

func (s *testSocket) Disconnect(close bool) {}
