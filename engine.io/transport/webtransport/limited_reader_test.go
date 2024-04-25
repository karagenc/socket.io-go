package webtransport

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLimitedReader(t *testing.T) {
	buf := bytes.NewBufferString("123456789")
	lr := newLimitedReader(buf, 5)

	// Exceeds 5
	p := make([]byte, 6)
	_, err := lr.Read(p)
	require.Error(t, err)

	// Doesn't exceed 5
	p = make([]byte, 3)
	n, err := lr.Read(p)
	require.Equal(t, nil, err)
	require.Equal(t, 3, n)
	require.Equal(t, "789", string(p[:n]))
}
