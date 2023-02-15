package fast

type SerializerType int

const (
	SerializerTypeSonic SerializerType = iota
	SerializerTypeGoJSON
)

func (t SerializerType) Name() string {
	switch t {
	case SerializerTypeSonic:
		return "sonic"
	case SerializerTypeGoJSON:
		return "go-json"
	}
	return "<invalid>"
}
