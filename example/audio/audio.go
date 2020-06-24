package audio

import (
	"errors"
	"time"
)

const (
	Ptime10ms = time.Millisecond * 10
	Ptime20ms = time.Millisecond * 20
	Ptime30ms = time.Millisecond * 30

	Ptime10 = 10
	Ptime20 = 20
	Ptime30 = 30
)

var (
	ErrPCMChunkNotFound = errors.New("PCM chunk not found")
	ErrPCMNot16Bit      = errors.New("PCM is not 16-bit")
	ErrClosed           = errors.New("closed")

	_Bufs = NewBufferPool()
)
