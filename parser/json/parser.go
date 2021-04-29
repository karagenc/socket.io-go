package jsonparser

import "github.com/tomruk/socket.io-go/parser"

type Creator struct {
	// Maximum number of the binary attachments to parse/send.
	// If this value is 0 (default), there will be no limit set for binary attachments.
	MaxAttachments int
}

// Make sure the Creator struct implements parser.Creator
var _ parser.Creator = &Creator{}

func (c *Creator) New() parser.Parser {
	return &Parser{
		maxAttachments: c.MaxAttachments,
	}
}

type Parser struct {
	r              *reconstructor
	maxAttachments int
}
