package audio

import (
	"fmt"
	"sync"
	"time"
)

// Pool contains logic of reusing byte slices of various size.
type Pool struct {
	_pool80   *bufPool
	_pool160  *bufPool
	_pool240  *bufPool
	_pool320  *bufPool
	_pool480  *bufPool
	_pool640  *bufPool
	_pool800  *bufPool
	_pool960  *bufPool
	_pool1060 *bufPool
	_pool1440 *bufPool
}

func NewBufferPool() *Pool {
	return &Pool{
		_pool80:   newBufPool(80),
		_pool160:  newBufPool(160),
		_pool240:  newBufPool(240),
		_pool320:  newBufPool(320),
		_pool480:  newBufPool(480),
		_pool640:  newBufPool(640),
		_pool800:  newBufPool(800),
		_pool960:  newBufPool(960),
		_pool1060: newBufPool(1060),
		_pool1440: newBufPool(1440),
	}
}

func (p *Pool) Get(clockSpeed int, ptime int) (BufferPool, error) {
	switch clockSpeed {
	case 8000:
		switch ptime {
		case Ptime10:
			return p._pool80, nil
		case Ptime20:
			return p._pool160, nil
		case Ptime30:
			return p._pool240, nil
		}
	case 16000:
		switch ptime {
		case Ptime10:
			return p._pool160, nil
		case Ptime20:
			return p._pool320, nil
		case Ptime30:
			return p._pool480, nil
		}
	case 32000:
		switch ptime {
		case Ptime10:
			return p._pool320, nil
		case Ptime20:
			return p._pool640, nil
		case Ptime30:
			return p._pool960, nil
		}
	case 48000:
		switch ptime {
		case Ptime10:
			return p._pool480, nil
		case Ptime20:
			return p._pool1060, nil
		case Ptime30:
			return p._pool1440, nil
		}
	}

	return nil, fmt.Errorf("no pool for clock_speed:%d ptime:%d", clockSpeed, ptime)
}

func PtimeDuration(ptime int) time.Duration {
	return time.Duration(ptime) * time.Millisecond
}

type noPool struct {
	bufferSize int
}

func (p noPool) Get() []int16 {
	return make([]int16, p.bufferSize)
}

func (noPool) Put(p []int16) {
}

type BufferPool interface {
	BufferSize() int

	Get() []int16

	Put(p []int16)
}

type bufPool struct {
	bufferSize int
	pool       sync.Pool
}

func newBufPool(size int) *bufPool {
	return &bufPool{
		bufferSize: size,

		pool: sync.Pool{New: func() interface{} {
			return make([]int16, size)
		}},
	}
}

func (p *bufPool) BufferSize() int {
	return p.bufferSize
}

func (p *bufPool) Get() []int16 {
	return p.pool.Get().([]int16)
}

func (b *bufPool) Put(p []int16) {
	if len(p) != b.bufferSize {
		p = p[:]
		if len(p) != b.bufferSize {
			return
		}
	}
	b.pool.Put(p)
}
