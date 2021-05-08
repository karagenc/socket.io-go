package eio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase64IDOutputLength(t *testing.T) {
	id, err := GenerateBase64ID(Base64IDSize)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 20, len(id), "invalid base64 ID length")
}

func TestBase64IDInvalidSize(t *testing.T) {
	_, err := GenerateBase64ID(1 /* something smaller than 4 */)
	assert.Equal(t, errBase64IDInvalidSize, err)
}
