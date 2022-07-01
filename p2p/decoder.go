package p2p

import (
	"github.com/golang/snappy"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protowire"
	"io"
	"math"
	"sync"
)

const maxVarintLength = 10

var errExcessMaxLength = errors.Errorf("provided header exceeds the max varint length of %d bytes", maxVarintLength)

var bufReaderPool = new(sync.Pool)
var bufWriterPool = new(sync.Pool)

// readVarint at the beginning of a byte slice. This varint may be used to indicate
// the length of the remaining bytes in the reader.
func readVarint(r io.Reader) (uint64, error) {
	b := make([]byte, 0, maxVarintLength)
	for i := 0; i < maxVarintLength; i++ {
		b1 := make([]byte, 1)
		n, err := r.Read(b1)
		if err != nil {
			return 0, err
		}
		if n != 1 {
			return 0, errors.New("did not read a byte from stream")
		}
		b = append(b, b1[0])

		// If most significant bit is not set, we have reached the end of the Varint.
		if b1[0]&0x80 == 0 {
			break
		}

		// If the varint is larger than 10 bytes, it is invalid as it would
		// exceed the size of MaxUint64.
		if i+1 >= maxVarintLength {
			return 0, errExcessMaxLength
		}
	}

	vi, n := protowire.ConsumeVarint(b)
	if n != len(b) {
		return 0, errors.New("varint did not decode entire byte slice")
	}
	return vi, nil
}

func writeVarint(n int) []byte {
	return protowire.AppendVarint(make([]byte, 0, maxVarintLength), uint64(n))
}

// Instantiates a new instance of the snappy buffered reader
// using our sync pool.
func newBufferedReader(r io.Reader) *snappy.Reader {
	rawReader := bufReaderPool.Get()
	if rawReader == nil {
		return snappy.NewReader(r)
	}
	bufR, ok := rawReader.(*snappy.Reader)
	if !ok {
		return snappy.NewReader(r)
	}
	bufR.Reset(r)
	return bufR
}

func maxLength(length uint64) (int, error) {
	if length > math.MaxInt {
		return 0, errors.New("invalid length provided")
	}

	il := int(length)
	maxLen := snappy.MaxEncodedLen(il)
	if maxLen < 0 {
		return 0, errors.Errorf("max encoded length is negative: %d", maxLen)
	}
	return maxLen, nil
}

func newBufferedWriter(w io.Writer) *snappy.Writer {
	rawBufWriter := bufWriterPool.Get()
	if rawBufWriter == nil {
		return snappy.NewBufferedWriter(w)
	}
	bufW, ok := rawBufWriter.(*snappy.Writer)
	if !ok {
		return snappy.NewBufferedWriter(w)
	}
	bufW.Reset(w)
	return bufW
}

func writeSnappyBuffer(w io.Writer, b []byte) (int, error) {
	bufWriter := newBufferedWriter(w)
	defer bufWriterPool.Put(bufWriter)
	num, err := bufWriter.Write(b)
	if err != nil {
		// Close buf writer in the event of an error.
		if err := bufWriter.Close(); err != nil {
			return 0, err
		}
		return 0, err
	}
	return num, bufWriter.Close()
}