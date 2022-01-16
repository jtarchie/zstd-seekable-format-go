package seekable

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
)

type bytesErr struct {
	tag           uint32
	input         []byte
	expectedBytes []byte
	expectedErr   error
}

func TestCreateSkippableFrame(t *testing.T) {
	dec, err := zstd.NewReader(nil)
	assert.NoError(t, err)

	for _, tab := range []bytesErr{
		{
			tag:           0x00,
			input:         []byte{},
			expectedBytes: nil,
			expectedErr:   nil,
		}, {
			tag:           0x01,
			input:         []byte{'T'},
			expectedBytes: []byte{0x51, 0x2a, 0x4d, 0x18, 0x01, 0x00, 0x00, 0x00, 'T'},
			expectedErr:   nil,
		}, {
			tag:           0xff,
			input:         []byte{'T'},
			expectedBytes: nil,
			expectedErr:   fmt.Errorf("requested tag (255) > 0xf"),
		},
	} {
		actualBytes, err := createSkippableFrame(tab.tag, tab.input)
		assert.Equal(t, tab.expectedErr, err, "createSkippableFrame err does not match expected")
		if tab.expectedErr == nil && err == nil {
			assert.Equal(t, tab.expectedBytes, actualBytes, "createSkippableFrame output does not match expected")
			decodedeBytes, err := dec.DecodeAll(actualBytes, nil)
			assert.NoError(t, err)
			assert.Equal(t, []byte(nil), decodedeBytes)
		}
	}
}

func TestWriter(t *testing.T) {
	var b bytes.Buffer
	bw := io.Writer(&b)
	w, err := NewWriter(bw)
	assert.NoError(t, err)

	bytes1 := []byte("test1")
	bytesWritten1, err := w.Write(bytes1)
	assert.NoError(t, err)
	bytes2 := []byte("test2")
	_, err = w.Write(bytes2)
	assert.NoError(t, err)

	err = w.Close()
	assert.NoError(t, err)

	// verify buffer content
	buf := b.Bytes()
	// magic footer
	assert.Equal(t, buf[len(buf)-4:], []byte{0xb1, 0xea, 0x92, 0x8f})
	assert.Equal(t, uint32(2), binary.LittleEndian.Uint32(buf[len(buf)-9:len(buf)-5]))
	// index.1
	indexOffset := len(buf) - 4 - 1 - 4 - 2*12
	assert.Equal(t, uint32(bytesWritten1), binary.LittleEndian.Uint32(buf[indexOffset:indexOffset+4]))
	assert.Equal(t, uint32(len(bytes1)), binary.LittleEndian.Uint32(buf[indexOffset+4:indexOffset+8]))
	// skipframe header
	frameOffset := indexOffset - 4 - 4
	assert.Equal(t, buf[frameOffset:frameOffset+4], []byte{0x5e, 0x2a, 0x4d, 0x18})
	assert.Equal(t, uint32(0x21), binary.LittleEndian.Uint32(buf[frameOffset+4:frameOffset+8]))

	// test decompression
	br := io.Reader(&b)
	dec, err := zstd.NewReader(br)
	assert.NoError(t, err)
	readBuf := make([]byte, 1024)
	n, err := dec.Read(readBuf)
	assert.Equal(t, err, io.EOF)
	concat := append(bytes1, bytes2...)
	assert.Equal(t, len(concat), n)
	assert.Equal(t, concat, readBuf[:n])
}
