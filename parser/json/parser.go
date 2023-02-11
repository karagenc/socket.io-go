package jsonparser

import (
	"github.com/tomruk/socket.io-go/parser"
)

// maxAttachments is the maximum number of the binary attachments to parse/send.
// If maxAttachments is 0, there will be no limit set for binary attachments.
func NewCreator(maxAttachments int, jsonAPI JSONAPI) parser.Creator {
	if jsonAPI == nil {
		panic("jsonparser.NewCreator: jsonAPI must be set")
	}
	return func() parser.Parser {
		return &Parser{
			maxAttachments: maxAttachments,
			json:           jsonAPI,
		}
	}
}

type Parser struct {
	r              *reconstructor
	maxAttachments int
	json           JSONAPI
}

func (p *Parser) Reset() {
	p.r = nil
}
