package websocket

import "nhooyr.io/websocket"

var expectedCloseCodes = []websocket.StatusCode{
	websocket.StatusNormalClosure,
	websocket.StatusGoingAway,
	websocket.StatusNoStatusRcvd,
	websocket.StatusAbnormalClosure,
}
