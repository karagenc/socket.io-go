package eio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase64IDOutputLength(t *testing.T) {
	id, err := generateBase64ID(base64IDSize)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 20, len(id), "invalid base64 ID length")
}

func TestBase64IDInvalidSize(t *testing.T) {
	_, err := generateBase64ID(1 /* something smaller than 4 */)
	assert.Equal(t, errBase64IDInvalidSize, err)
}
