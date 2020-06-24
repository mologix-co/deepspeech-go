package audio

import (
	"io"
	"sync"
	"time"
)

// Buffer between Reader and Writer. Once internal frame buffer is full, it blocks
// until either the Reader or Writer processes the next frame.
type Buffer struct {
	sampleRate int
	ptime      int
	pool       BufferPool

	eof bool

	max    int
	frames [][]int16
	ridx   int
	widx   int
	size   int

	samplesRead    int
	sampleDuration time.Duration

	closed     bool
	readerWait bool
	writerWait bool
	readerWg   sync.WaitGroup
	writerWg   sync.WaitGroup
	mu         sync.Mutex
}

func NewBuffer(sampleRate, ptime, maxFrames int) (*Buffer, error) {
	pool, err := _Bufs.Get(16000, Ptime10)
	if err != nil {
		return nil, err
	}
	b := &Buffer{
		sampleRate:     sampleRate,
		ptime:          ptime,
		pool:           pool,
		eof:            false,
		max:            maxFrames,
		frames:         make([][]int16, maxFrames),
		samplesRead:    0,
		sampleDuration: time.Second / time.Duration(sampleRate),
		ridx:           0,
		widx:           0,
		size:           0,
		mu:             sync.Mutex{},
	}
	return b, nil
}

func (b *Buffer) Elapsed() time.Duration {
	return time.Duration(b.samplesRead) * b.sampleDuration
}

func (f *Buffer) SampleRate() int {
	return f.sampleRate
}

func (f *Buffer) FrameSize() int {
	return f.pool.BufferSize()
}

func (f *Buffer) Ptime() time.Duration {
	return time.Duration(f.ptime) * time.Millisecond
}

func (f *Buffer) Alloc() []int16 {
	return f.pool.Get()
}

func (f *Buffer) Release(b []int16) {
	f.pool.Put(b)
}

func (f *Buffer) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return io.ErrClosedPipe
	}
	f.closed = true
	if f.readerWait {
		f.readerWait = false
		// Unblock readers.
		f.readerWg.Done()
	}
	if f.writerWait {
		// Unblock writer.
		f.writerWait = false
		f.writerWg.Done()
	}
	// Release frames.
	for i, buf := range f.frames {
		f.pool.Put(buf)
		f.frames[i] = nil
	}
	f.frames = nil
	f.mu.Unlock()
	return nil
}

func (f *Buffer) WriteFinal() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return io.ErrClosedPipe
	}
	f.eof = true
	if f.writerWait {
		f.writerWait = false
		f.writerWg.Done()
	}
	if f.readerWait {
		f.readerWait = false
		f.readerWg.Done()
	}
	f.mu.Unlock()
	return nil
}

func (f *Buffer) Write(p []int16) error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return io.ErrClosedPipe
	}
	if f.eof {
		f.mu.Unlock()
		return io.EOF
	}
	if f.size == len(f.frames) {
		f.mu.Unlock()
		return io.ErrShortBuffer
	}

	f.frames[f.widx%len(f.frames)] = p
	f.widx++
	f.size++

	// Unblock reader.
	if f.readerWait {
		f.readerWait = false
		f.readerWg.Done()
	}
	f.mu.Unlock()
	return nil
}

func (f *Buffer) UnblockWriter() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writerWait {
		f.writerWait = false
		f.writerWg.Done()
		return true
	}
	return false
}

func (f *Buffer) WriteBlocking(p []int16) error {
	f.mu.Lock()
	// Was it recently closed.
	if f.closed {
		f.mu.Unlock()
		return io.ErrClosedPipe
	}
	if f.eof {
		f.mu.Unlock()
		return io.EOF
	}
	if f.size == f.max {
		// Wait for reader to read next frame.
		if !f.writerWait {
			f.writerWait = true
			f.writerWg.Add(1)
		}
		f.mu.Unlock()
		// Wait for next ReadFrame to free up a slot.
		f.writerWg.Wait()
		// Try Non-Blocking write. This may return error.
		return f.Write(p)
	}
	f.frames[f.widx%f.max] = p
	f.widx++
	f.size++

	if f.readerWait {
		f.readerWait = false
		f.readerWg.Done()
	}

	f.mu.Unlock()
	return nil
}

func (f *Buffer) ReadFrame() ([]int16, error) {
	for {
		f.mu.Lock()
		if f.closed {
			f.mu.Unlock()
			return nil, io.ErrClosedPipe
		}

		if f.size == 0 {
			if f.eof {
				f.mu.Unlock()
				return nil, io.EOF
			}

			// Wait for next write.
			if !f.readerWait {
				f.readerWait = true
				f.readerWg.Add(1)
			}
			f.mu.Unlock()
			f.readerWg.Wait()
			continue
		}

		buf := f.frames[f.ridx%f.max]
		f.ridx++
		f.size--
		f.samplesRead += len(buf)

		// Notify writer if needed.
		if f.writerWait {
			f.writerWait = false
			f.writerWg.Done()
		}
		f.mu.Unlock()

		return buf, nil
	}
}
