package jsonparser

import (
	"bytes"
	"reflect"

	"fmt"
	"strconv"

	"github.com/karagenc/socket.io-go/parser"
)

var (
	errInvalidPacketSize          = fmt.Errorf("parser/json: invalid packet size")
	errMalformedPacket            = fmt.Errorf("parser/json: malformed packet")
	errInvalidPlaceholderNumValue = fmt.Errorf("parser/json: invalid placeholder num value")
	errInvalidNumberOfBuffers     = fmt.Errorf("parser/json: invalid number of buffers")
	errInvalidNumberOfValues      = fmt.Errorf("parser/json: invalid number of values")
)

var stringType = reflect.TypeOf("")

func (p *Parser) Add(data []byte, finish parser.Finish) error {
	if p.r == nil {
		header, buf, eventName, err := p.parseHeader(data)
		if err != nil {
			return err
		}

		p.r = &reconstructor{
			header:    header,
			eventName: eventName,
			buffers:   [][]byte{buf},
			remaining: header.Attachments,
			json:      p.json,
		}

		if p.maxAttachments > 0 && header.Attachments > p.maxAttachments {
			return errMaxAttachmentsExceeded
		}

		ok := !header.IsBinary() || header.Attachments == 0
		if ok {
			r := p.r
			p.r = nil
			finish(header, eventName, r.decode)
		}
		return nil
	}

	ok := p.r.addBuffer(data)
	if ok {
		r := p.r
		p.r = nil
		finish(r.header, r.eventName, r.decode)
	}
	return nil
}

func (p *Parser) parseHeader(data []byte) (header *parser.PacketHeader, buf []byte, eventName string, err error) {
	if len(data) < 1 {
		err = errInvalidPacketSize
		return
	}

	header = new(parser.PacketHeader)

	err = header.Type.FromChar(data[0])
	if err != nil {
		return
	}
	data = data[1:]

	// If packet type is binary, look up attachments
	if header.IsBinary() {
		i := bytes.IndexByte(data, '-')
		if i == -1 {
			err = errMalformedPacket
			return
		}

		attachments, err := strconv.ParseUint(string(data[:i]), 10, 0)
		if err != nil {
			return nil, nil, "", err
		}

		header.Attachments = int(attachments)

		if i+1 < len(data) {
			data = data[i+1:]
		} else {
			data = data[i:]
		}
	}

	// Look up namespace
	if len(data) >= 1 && data[0] == '/' {
		i := 0
		for ; ; i++ {
			if i == len(data) {
				break
			} else if data[i] == ',' {
				break
			}
		}

		header.Namespace = string(data[:i])
		data = data[i+1:]
	} else {
		header.Namespace = "/"
	}

	// Look up ID
	if len(data) >= 1 && data[0] >= '0' && data[0] <= '9' {
		i := 0
		for ; ; i++ {
			if i == len(data) {
				break
			} else if data[i] < '0' || data[i] > '9' {
				break
			}
		}

		num, err := strconv.ParseUint(string(data[:i]), 10, 0)
		if err != nil {
			return nil, nil, "", err
		}
		header.ID = &num
		data = data[i:]
	}

	if header.IsEvent() {
		start := 0
		found := false

		for ; start < len(data); start++ {
			c := data[start]
			if c == '"' {
				found = true
				break
			}
		}

		if !found {
			err = errMalformedPacket
			return
		}

		var tmp []byte
		end := start + 1
		found = false

		for ; end < len(data); end++ {
			c := data[end]
			if c == '"' && data[end-1] != '\\' {
				b := data[start : end+1]

				tmp = make([]byte, len(b)+2)
				tmp[0] = '['
				tmp[len(b)+1] = ']'

				for i := 0; i < len(b); i++ {
					tmp[i+1] = b[i]
				}

				found = true
				break
			}
		}

		if !found {
			err = errMalformedPacket
			return
		}

		var v []string
		err = p.json.Unmarshal(tmp, &v)
		if err != nil {
			return
		}

		if len(v) != 1 {
			err = errMalformedPacket
			return
		}
		eventName = v[0]
	}

	buf = data
	return
}

func (r *reconstructor) decode(types ...reflect.Type) (values []reflect.Value, err error) {
	// We have no binary data.
	if len(r.buffers) == 1 {
		payload := r.buffers[0]
		values = convertTypesToValues(types...)

		if r.header.IsEvent() {
			eventName := reflect.New(stringType)
			values = append([]reflect.Value{eventName}, values...)

			if len(payload) == 0 {
				return nil, errMalformedPacket
			}

			ifaces := make([]any, len(values))

			for i, rv := range values {
				if !rv.CanInterface() {
					return nil, &ValueError{err: errNonInterfaceableValue, Value: rv}
				}

				ifaces[i] = rv.Interface()
			}

			if len(payload) == 0 {
				payload = []byte("[]")
			}

			err = r.json.Unmarshal(payload, &ifaces)
			if err != nil {
				return nil, err
			}

			values = values[1:]
			return
		}

		if len(values) == 1 && !r.header.IsAck() {
			rv := values[0]
			if !rv.CanInterface() {
				return nil, &ValueError{err: errNonInterfaceableValue, Value: rv}
			}

			if len(payload) == 0 {
				payload = []byte("{}")
			}

			err = r.json.Unmarshal(payload, rv.Interface())
			if err != nil {
				return nil, err
			}
		} else {
			ifaces := make([]any, len(values))

			for i, rv := range values {
				if !rv.CanInterface() {
					return nil, &ValueError{err: errNonInterfaceableValue, Value: rv}
				}

				ifaces[i] = rv.Interface()
			}

			if len(payload) == 0 {
				payload = []byte("[]")
			}

			err = r.json.Unmarshal(payload, &ifaces)
			if err != nil {
				return nil, err
			}
		}

		return
	}

	return r.reconstruct(types...)
}

func convertTypesToValues(types ...reflect.Type) (values []reflect.Value) {
	values = make([]reflect.Value, len(types))

	for i, typ := range types {
		if typ == nil {
			var (
				unused any
				ptr    = &unused
			)
			typ = reflect.TypeOf(ptr)
		}

		k := typ.Kind()
		if k == reflect.Ptr {
			typ = typ.Elem()
			//k = typ.Kind()
		}

		values[i] = reflect.New(typ)
	}
	return
}
