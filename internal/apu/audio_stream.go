package apu

type AudioStream struct {
	buffer []byte
	r      int // read position
	w      int // write position
}

func NewAudioStream(size int) *AudioStream {
	return &AudioStream{
		buffer: make([]byte, size),
	}
}

// The Read is the Implementation of io.Reader.Read().
func (as *AudioStream) Read(p []byte) (int, error) {
	n := 0
	for i := range p {
		p[i] = as.buffer[as.r]
		as.r = (as.r + 1) % len(as.buffer)
		n++
	}
	return n, nil
}

// The write is only used in APU.
func (as *AudioStream) write(p [8]byte) {
	for _, b := range p {
		as.buffer[as.w] = b
		as.w = (as.w + 1) % len(as.buffer)
	}
}
