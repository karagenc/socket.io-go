package jsonparser

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/tomruk/socket.io-go/parser"
)

var (
	errInvalidPlaceholder = fmt.Errorf("parser/json: invalid placeholder")
	errBinaryCannotBeAPtr = fmt.Errorf("parser/json: sio.Binary cannot be a pointer")
)

type socketIOBinary interface {
	SocketIOBinary() bool
}

type Binary []byte

func (b Binary) SocketIOBinary() bool {
	return true
}

func (b Binary) MarshalJSON() ([]byte, error) {
	if b == nil {
		return []byte("null"), nil
	}
	return b, nil
}

func (b *Binary) UnmarshalJSON(data []byte) error {
	if b == nil {
		return errors.New("sio.Binary: UnmarshalJSON on nil pointer")
	}
	*b = append((*b)[0:0], data...)
	return nil
}

type placeholder struct {
	Placeholder bool `json:"_placeholder"`
	Num         int  `json:"num"`
}

func (p *Parser) deconstructPacket(rv reflect.Value, numBuffers *int) (buffers [][]byte, err error) {
	return p.deconstructValue(rv, numBuffers)
}

func (p *Parser) deconstructValue(rv reflect.Value, numBuffers *int) (buffers [][]byte, err error) {
	k := rv.Kind()
	original := rv
	if k == reflect.Interface || k == reflect.Ptr {
		rv = rv.Elem()
		k = rv.Kind()
	}

	// Check twice. rv can be a pointer to an interface.
	if k == reflect.Interface || k == reflect.Ptr {
		rv = rv.Elem()
		k = rv.Kind()
	}

	switch k {
	case reflect.Slice:
		sk := rv.Type().Elem().Kind()

		switch sk {
		case reflect.Ptr, reflect.Interface, reflect.Struct, reflect.Slice:
			sl := rv.Len()
			for i := 0; i < sl; i++ {
				el := rv.Index(i)
				b, err := p.deconstructValue(el, numBuffers)
				if err != nil {
					return nil, err
				}
				buffers = append(buffers, b...)
			}

		case reflect.Uint8:
			switch original.Kind() {
			case reflect.Interface:
				orig := original.Elem()
				if orig.Kind() == reflect.Ptr {
					return nil, errBinaryCannotBeAPtr
				}
			case reflect.Ptr:
				return nil, errBinaryCannotBeAPtr
			}

			buf, err := p.deconstructBinaryValue(rv, original, numBuffers, nil)
			if err != nil {
				return nil, err
			}
			buffers = append(buffers, buf)
		}

	case reflect.Struct:
		// If rv is non-settable, we copy it and assign the pointer of the copy to the original.
		if !rv.CanSet() && original.Kind() == reflect.Interface && original.CanSet() {
			ne := reflect.New(rv.Type())
			el := ne.Elem()
			el.Set(rv)
			original.Set(ne)
			rv = el
		}

		b, err := p.deconstructStruct(rv, numBuffers)
		if err != nil {
			return nil, err
		}
		buffers = append(buffers, b...)

	case reflect.Map:
		b, err := p.deconstructMap(rv, numBuffers)
		if err != nil {
			return nil, err
		}
		buffers = append(buffers, b...)
	}

	return
}

func (p *Parser) deconstructBinaryValue(rv reflect.Value, original reflect.Value, numBuffers *int, customSetter func([]byte) error) (buf []byte, err error) {
	if rv.CanInterface() {
		sb, ok := rv.Interface().(socketIOBinary)
		if ok && sb.SocketIOBinary() == true {
			buf = rv.Bytes()

			phold := placeholder{
				Placeholder: true,
				Num:         *numBuffers,
			}
			*numBuffers++

			pBuf, err := p.json.Marshal(&phold)
			if err != nil {
				return nil, err
			}

			if customSetter != nil {
				err = customSetter([]byte(pBuf))
				if err != nil {
					return nil, err
				}
			} else if rv.CanSet() {
				rv.SetBytes([]byte(pBuf))
			} else {
				if !original.CanSet() {
					return nil, &ValueError{err: errNonSettableValue, Value: rv}
				}

				n := reflect.MakeSlice(rv.Type(), len(pBuf), len(pBuf))
				b := n.Bytes()

				for i := 0; i < len(b); i++ {
					b[i] = pBuf[i]
				}

				x := reflect.New(rv.Type())
				x.Elem().Set(n)
				original.Set(x)
			}
		}
	}

	return
}

func (p *Parser) deconstructStruct(rv reflect.Value, numBuffers *int) (buffers [][]byte, err error) {
	nf := rv.NumField()

	for i := 0; i < nf; i++ {
		fv := rv.Field(i)

		k := fv.Kind()
		if k == reflect.Interface || k == reflect.Ptr {
			fv = fv.Elem()
			//k = fv.Kind()
		}

		if !fv.IsValid() || !fv.CanInterface() {
			continue
		}

		b, err := p.deconstructValue(fv, numBuffers)
		if err != nil {
			return nil, err
		}
		buffers = append(buffers, b...)
	}

	return
}

func (p *Parser) deconstructMap(rv reflect.Value, numBuffers *int) (buffers [][]byte, err error) {
	iter := rv.MapRange()
	for iter.Next() {
		mk := iter.Key()
		mv := iter.Value()
		original := mv
		k := mv.Kind()

		if k == reflect.Interface || k == reflect.Ptr {
			mv = mv.Elem()
			k = mv.Kind()
		}

		if k == reflect.Slice && mv.Type().Elem().Kind() == reflect.Uint8 {
			set := func(buf []byte) error {
				n := reflect.MakeSlice(mv.Type(), len(buf), len(buf))
				b := n.Bytes()

				for i := 0; i < len(b); i++ {
					b[i] = buf[i]
				}

				x := reflect.New(mv.Type())
				x.Elem().Set(n)
				rv.SetMapIndex(mk, x)
				return nil
			}

			buf, err := p.deconstructBinaryValue(mv, original, numBuffers, set)
			if err != nil {
				return nil, err
			}
			buffers = append(buffers, buf)
			continue
		}

		b, err := p.deconstructValue(mv, numBuffers)
		if err != nil {
			return nil, err
		}
		buffers = append(buffers, b...)
	}

	return
}

type reconstructor struct {
	header    *parser.PacketHeader
	eventName string
	buffers   [][]byte
	remaining int
}

func (r *reconstructor) AddBuffer(buf []byte) (ok bool) {
	r.buffers = append(r.buffers, buf)
	r.remaining--
	return r.remaining == 0
}

func (r *reconstructor) Reconstruct(types ...reflect.Type) (values []reflect.Value, err error) {
	if len(r.buffers) < 1 {
		return nil, errInvalidNumberOfBuffers
	}

	payload := r.buffers[0]
	values = convertTypesToValues(types...)

	eventName := reflect.New(stringType)
	values = append([]reflect.Value{eventName}, values...)

	if r.header.IsEvent() && len(values) == 0 {
		return nil, errInvalidNumberOfValues
	}

	ifaces := make([]interface{}, len(values))

	for i, rv := range values {
		if !rv.CanInterface() {
			return nil, &ValueError{err: errNonInterfaceableValue, Value: rv}
		}

		ifaces[i] = rv.Interface()
	}

	err = json.Unmarshal(payload, &ifaces)
	if err != nil {
		return
	}

	if r.header.IsEvent() {
		values = values[1:]
	}

	err = r.reconstructPacket(values)
	return
}

func (r *reconstructor) reconstructPacket(rv []reflect.Value) error {
	for _, rv := range rv {
		err := r.reconstructValue(rv)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *reconstructor) reconstructValue(rv reflect.Value) error {
	k := rv.Kind()
	original := rv
	if k == reflect.Interface || k == reflect.Ptr {
		rv = rv.Elem()
		k = rv.Kind()
	}

	// Check twice. rv can be a pointer to an interface.
	if k == reflect.Interface || k == reflect.Ptr {
		rv = rv.Elem()
		k = rv.Kind()
	}

	switch k {
	case reflect.Slice:
		sk := rv.Type().Elem().Kind()

		switch sk {
		case reflect.Ptr, reflect.Interface, reflect.Struct, reflect.Slice:
			sl := rv.Len()
			for i := 0; i < sl; i++ {
				el := rv.Index(i)
				err := r.reconstructValue(el)
				if err != nil {
					return err
				}
			}

		case reflect.Uint8:
			err := r.reconstructBinaryValue(rv, original, nil)
			if err != nil {
				return err
			}
		}

	case reflect.Struct:
		err := r.reconstructStruct(rv)
		if err != nil {
			return err
		}

	case reflect.Map:
		err := r.reconstructMap(rv)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *reconstructor) reconstructBinaryValue(rv reflect.Value, original reflect.Value, customSetter func([]byte) error) error {
	if rv.CanInterface() {
		sb, ok := rv.Interface().(socketIOBinary)
		if ok && sb.SocketIOBinary() == true {
			pBuf := rv.Bytes()

			var p placeholder
			err := json.Unmarshal(pBuf, &p)
			if err != nil {
				return err
			}

			num := p.Num + 1

			if num >= len(r.buffers) {
				return errInvalidPlaceholderNumValue
			}

			buf := r.buffers[num]

			if customSetter != nil {
				err = customSetter(buf)
				if err != nil {
					return err
				}
			} else if rv.CanSet() {
				rv.SetBytes(buf)
			} else {
				if !original.CanSet() {
					return &ValueError{err: errNonSettableValue, Value: rv}
				}

				n := reflect.MakeSlice(rv.Type(), len(buf), len(buf))
				b := n.Bytes()

				for i := 0; i < len(b); i++ {
					b[i] = buf[i]
				}

				x := reflect.New(rv.Type())
				x.Elem().Set(n)
				original.Set(x)
			}
		}
	}

	return nil
}

func (r *reconstructor) reconstructStruct(rv reflect.Value) error {
	nf := rv.NumField()

	for i := 0; i < nf; i++ {
		fv := rv.Field(i)

		k := fv.Kind()
		if k == reflect.Interface || k == reflect.Ptr {
			fv = fv.Elem()
			//k = fv.Kind()
		}

		if !fv.IsValid() || !fv.CanInterface() {
			continue
		}

		err := r.reconstructValue(fv)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *reconstructor) reconstructMap(rv reflect.Value) error {
	iter := rv.MapRange()
	for iter.Next() {
		mk := iter.Key()
		mv := iter.Value()
		original := mv
		k := mv.Kind()

		if k == reflect.Interface || k == reflect.Ptr {
			mv = mv.Elem()
			k = mv.Kind()
		}

		switch k {
		case reflect.Map:
			mapKeys := mv.MapKeys()

			if rv.Type().Elem().Kind() == reflect.Interface && len(mapKeys) == 2 && mv.Type().Key().Kind() == reflect.String {
				if mapKeys[0].String() == "_placeholder" && mapKeys[1].String() == "num" {
					pholder := mv.MapIndex(mapKeys[0])
					num := mv.MapIndex(mapKeys[1])

					if pholder.Kind() == reflect.Interface {
						pholder = pholder.Elem()
					}
					if num.Kind() == reflect.Interface {
						num = num.Elem()
					}

					if pholder.Kind() == reflect.Bool && pholder.Bool() == true && num.Kind() == reflect.Float64 {
						n := int(num.Float())
						n++

						if n >= len(r.buffers) {
							return errInvalidPlaceholderNumValue
						}

						buf := r.buffers[n]
						rv.SetMapIndex(mk, reflect.ValueOf(buf))
						continue
					}

				} else if mapKeys[1].String() == "_placeholder" && mapKeys[0].String() == "num" {
					pholder := mv.MapIndex(mapKeys[1])
					num := mv.MapIndex(mapKeys[0])

					if pholder.Kind() == reflect.Interface {
						pholder = pholder.Elem()
					}
					if num.Kind() == reflect.Interface {
						num = num.Elem()
					}

					if pholder.Kind() == reflect.Bool && pholder.Bool() == true && num.Kind() == reflect.Float64 {
						n := int(num.Float())
						n++

						if n >= len(r.buffers) {
							return errInvalidPlaceholderNumValue
						}

						buf := r.buffers[n]
						rv.SetMapIndex(mk, reflect.ValueOf(buf))
						continue
					}
				}
			}

			err := r.reconstructValue(mv)
			if err != nil {
				return err
			}

		case reflect.Slice:
			if k == reflect.Slice && mv.Type().Elem().Kind() == reflect.Uint8 {
				set := func(buf []byte) error {
					n := reflect.MakeSlice(mv.Type(), len(buf), len(buf))
					b := n.Bytes()

					for i := 0; i < len(b); i++ {
						b[i] = buf[i]
					}

					x := reflect.New(mv.Type())
					x.Elem().Set(n)
					rv.SetMapIndex(mk, x)
					return nil
				}

				err := r.reconstructBinaryValue(mv, original, set)
				if err != nil {
					return err
				}
				continue
			}

		default:
			err := r.reconstructValue(mv)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func hasBinary(values ...reflect.Value) bool {
	for _, rv := range values {
		k := rv.Kind()
		if k == reflect.Interface || k == reflect.Ptr {
			rv = rv.Elem()
			k = rv.Kind()
		}

		// Check twice. rv can be a pointer to an interface.
		if k == reflect.Interface || k == reflect.Ptr {
			rv = rv.Elem()
			k = rv.Kind()
		}

		switch k {
		case reflect.Slice:
			sk := rv.Type().Elem().Kind()

			switch sk {
			case reflect.Ptr, reflect.Interface, reflect.Struct, reflect.Slice:
				l := rv.Len()
				for i := 0; i < l; i++ {
					val := rv.Index(i)
					if hasBinary(val) {
						return true
					}
				}
			case reflect.Uint8:
				if rv.CanInterface() {
					sb, ok := rv.Interface().(socketIOBinary)
					if ok && sb.SocketIOBinary() == true {
						return true
					}
				}
			}

		case reflect.Struct:
			nf := rv.NumField()
			for i := 0; i < nf; i++ {
				fv := rv.Field(i)

				if !fv.IsValid() || !fv.CanInterface() {
					continue
				}

				if hasBinary(fv) {
					return true
				}
			}

		case reflect.Map:
			iter := rv.MapRange()
			for iter.Next() {
				mv := iter.Value()
				if hasBinary(mv) {
					return true
				}
			}
		}
	}

	return false
}
