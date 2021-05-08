package parser

import "reflect"

type Creator func() Parser

type Finish func(header *PacketHeader, eventName string, decode Decode)

type Decode func(types ...reflect.Type) (values []reflect.Value, err error)

type Parser interface {
	Encode(header *PacketHeader, v interface{}) (buffers [][]byte, err error)
	Add(data []byte, finish Finish) error
}
