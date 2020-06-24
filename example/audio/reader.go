package audio

import (
	"io"
	"time"
)

// 16bit PCM reader.
type Reader interface {
	io.Closer

	Elapsed() time.Duration

	// Clock speed in hertz. (i.e. 16000 for 16Khz)
	SampleRate() int

	// Number of 16bit integers per frame.
	FrameSize() int

	// Frame duration.
	Ptime() time.Duration

	// Release buffer to allow it to be recycled.
	Release(p []int16)

	// Allocate a new Frame.
	Alloc() []int16

	// ReadFrame the next Frame.
	ReadFrame() ([]int16, error)
}

type RawReader interface {
	Read(p []int16) (n int, err error)
}
