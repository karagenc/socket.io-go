package jsonparser

import (
	"reflect"
	"testing"

	"github.com/karagenc/socket.io-go/parser"
	"github.com/karagenc/socket.io-go/parser/json/serializer/stdjson"
)

func TestDecode(t *testing.T) {
	c := NewCreator(0, stdjson.New())
	p := c()
	tests := createDecodeTests(t)

	for _, test := range tests {
		finishHappened := false

		for _, buf := range test.Buffers {
			finishHappened = false

			finish := func(header *parser.PacketHeader, eventName string, decode parser.Decode) {
				values, err := decode(test.ExpectedTypes...)
				if err != nil {
					t.Fatalf("decode error: %v", err)
				}

				printValues(t, values...)

				values, err = decode(test.ExpectedTypes...)
				if err != nil {
					t.Fatalf("decode error: %v", err)
				}

				printValues(t, values...)

				finishHappened = true
			}

			err := p.Add(buf, finish)
			if err != nil {
				t.Fatalf("p.Add failed: %v", err)
			}
		}

		if !finishHappened {
			t.Fatalf("finish callback didn't run")
		}
	}
}

func TestMaxAttachmentsDecode(t *testing.T) {
	c := NewCreator(0, stdjson.New())
	p := c()

	header := &parser.PacketHeader{
		Type: parser.PacketTypeEvent,
	}

	v := struct {
		A1 Binary `json:"a1"`
		A2 Binary `json:"a2"`
		A3 Binary `json:"a3"`
		A4 Binary `json:"a4"`
	}{
		A1: Binary("a1"),
		A2: Binary("a2"),
		A3: Binary("a3"),
		A4: Binary("a4"),
	}

	buffers, err := p.Encode(header, &v)
	if err != nil {
		t.Fatal(err)
	}

	// Empty
	finish := func(header *parser.PacketHeader, eventName string, decode parser.Decode) {}

	c = NewCreator(3, stdjson.New())
	p = c()

	err = p.Add(buffers[0], finish)
	if err == nil {
		t.Fatal("error expected")
	}
}

func printValues(t *testing.T, values ...reflect.Value) {
	for i, rv := range values {
		k := rv.Kind()
		if k == reflect.Interface || k == reflect.Ptr {
			rv = rv.Elem()
			//k = rv.Kind()
		}

		v := rv.Interface()

		switch val := v.(type) {
		case Binary, *Binary:
			t.Logf("Data %d: %s", i, val)
		default:
			t.Logf("Data %d: %v", i, v)
		}
	}
	t.Logf("\n")
}

func createDecodeTests(t *testing.T) []*decodeTest {
	return []*decodeTest{
		{
			Buffers:        createBuffers([]byte("0")),
			ExpectedHeader: mustCreatePacketHeader(t, parser.PacketTypeConnect, "/", 0),
			ExpectedTypes:  nil,
		},
		{
			Buffers:        createBuffers([]byte(`2["evvvent",{"foo": "bar"}]`)),
			ExpectedHeader: mustCreatePacketHeader(t, parser.PacketTypeEvent, "/", 0),
			ExpectedTypes: createTypes(
				struct {
					Foo string `json:"foo"`
				}{},
			),
		},
	}
}

type decodeTest struct {
	Buffers        [][]byte
	ExpectedHeader *parser.PacketHeader
	ExpectedTypes  []reflect.Type
}

func createBuffers(payload []byte, attachments ...[]byte) [][]byte {
	return append([][]byte{payload}, attachments...)
}

func createTypes(v ...any) (types []reflect.Type) {
	for _, x := range v {
		typ := reflect.TypeOf(x)
		types = append(types, typ)
	}
	return
}
