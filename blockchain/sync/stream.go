package sync

import (
	"context"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"strings"
	"time"
)

var (
	respTimeout = time.Duration(params.RaidoConfig().ResponseTimeout) * time.Second
	writeDuration = respTimeout
)

const (
	readDuration = ttfbTimeout
)

const (
	codeSuccess         = byte(0x00)
	codeInternalError   = byte(0x01)
	codeValidationError = byte(0x02)
)

type streamHandler func(context.Context, interface{}, network.Stream) error

func isValidStreamError(err error) bool {
	// check the error message itself as well as libp2p doesn't currently
	// return the correct error type from Close{Read,Write,}.
	return err != nil && !errors.Is(err, network.ErrReset)
}

func closeStream(stream network.Stream) {
	if err := stream.Close(); isValidStreamError(err) {
		log.Error(err)
	}
}

func setStreamDeadlines(stream network.Stream) {
	SetReadDeadline(stream, readDuration)
	SetWriteDeadline(stream)
}

func SetReadDeadline(stream network.Stream, dur time.Duration) {
	err := stream.SetReadDeadline(time.Now().Add(dur))
	if err != nil && !strings.Contains(err.Error(), "stream closed") {
		log.Error(err)
	}
}

func SetWriteDeadline(stream network.Stream) {
	err := stream.SetWriteDeadline(time.Now().Add(writeDuration))
	if err != nil {
		log.Error(err)
	}
}

func ReadStatusCode(stream network.Stream) (uint8, string, error) {
	// Set ttfb deadline.
	setStreamDeadlines(stream)
	b := make([]byte, 1)
	_, err := stream.Read(b)
	if err != nil {
		return 0, "", err
	}

	var msg string
	switch b[0] {
	case codeSuccess:
		return 0, "", nil
	case codeInternalError:
		msg = "Internal peer error"
	case codeValidationError:
		msg = "Bad request given"
	}

	return b[0], msg, nil
}

func writeCodeToStream(stream network.Stream, code byte) {
	_, err := stream.Write([]byte{code})
	if err != nil {
		log.Errorf("Can't write response to the stream: %s", err)
	}
}