package jsonparser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/tomruk/socket.io-go/parser"
)

var (
	errNumBuffers             = fmt.Errorf("parser/json: numBuffers was expected to be equal to len(buffers)")
	errMaxAttachmentsExceeded = fmt.Errorf("parser/json: maximum number of attachments exceeded")
)

var (
	errNilArgument    = fmt.Errorf("nil argument provided")
	errNonPtrArgument = fmt.Errorf("the argument must be a pointer")
)

// A placeholder for an empty value.
type _empty struct{}

// A placeholder for an empty value.
var empty _empty

func (p *Parser) Encode(header *parser.PacketHeader, v interface{}) ([][]byte, error) {
	if v == nil {
		v = &empty
	}

	rv := reflect.ValueOf(v)

	if !rv.IsValid() {
		return nil, fmt.Errorf("parser/json: invalid argument: %w", errNilArgument)
	}

	k := rv.Kind()
	if k != reflect.Ptr && k != reflect.Struct /* Struct is an exception */ {
		return nil, fmt.Errorf("parser/json: invalid argument (%s): %w", rv.Type().String(), errNonPtrArgument)
	} else if k == reflect.Ptr && rv.IsNil() {
		return nil, fmt.Errorf("parser/json: invalid argument: %w", errNilArgument)
	}

	if header.Type == parser.PacketTypeEvent || header.Type == parser.PacketTypeAck {
		if hasBinary(rv) {
			if header.Type == parser.PacketTypeEvent {
				header.Type = parser.PacketTypeBinaryEvent
			} else if header.Type == parser.PacketTypeAck {
				header.Type = parser.PacketTypeBinaryAck
			}

			return p.encodeBinary(header, v)
		}
	}

	buf, err := p.encodeString(header, v)
	return [][]byte{buf}, err
}

func (p *Parser) encodeString(header *parser.PacketHeader, v interface{}) ([]byte, error) {
	var (
		buf  = bytes.Buffer{}
		e    = json.NewEncoder(&buf)
		grow int
	)

	grow += 1  // Packet type
	grow += 2  // Attachments
	grow += 20 // Namespace (Approximate length)
	grow += 20 // Ack ID (Max length)
	buf.Grow(grow)

	buf.WriteByte(header.Type.ToChar())

	if header.Type == parser.PacketTypeBinaryEvent || header.Type == parser.PacketTypeBinaryAck {
		buf.WriteString(strconv.Itoa(header.Attachments) + "-")
	}

	if header.Namespace != "" && header.Namespace != "/" {
		buf.WriteString(header.Namespace + ",")
	}

	if header.ID != nil {
		buf.WriteString(strconv.FormatUint(*header.ID, 10))
	}

	switch v.(type) {
	case _empty, *_empty:
		// Omit JSON.
	default:
		err := e.Encode(v)
		if err != nil {
			return nil, err
		}

		// Remove newline
		b := buf.Bytes()
		if len(b) != 0 && b[len(b)-1] == '\n' {
			b = b[:len(b)-1]
		}
		return b, nil
	}

	return buf.Bytes(), nil
}

func (p *Parser) encodeBinary(header *parser.PacketHeader, v interface{}) (buffers [][]byte, err error) {
	numBuffers := 0
	buffers, err = deconstructPacket(reflect.ValueOf(v), &numBuffers)
	if err != nil {
		return nil, err
	}

	if numBuffers != len(buffers) {
		return nil, errNumBuffers
	}

	if p.maxAttachments > 0 && numBuffers > p.maxAttachments {
		return nil, errMaxAttachmentsExceeded
	}

	header.Attachments = numBuffers

	s, err := p.encodeString(header, v)
	if err != nil {
		return nil, err
	}

	buffers = append([][]byte{s}, buffers...)
	return
}
