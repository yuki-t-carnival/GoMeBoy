package apu

type AudioStream struct {
	buffer []byte
	r      int // read position
	w      int // write position
	stock  int

	// emulator log
	writeCnt    int
	readCnt     int
	fillZeroCnt int
	skipCnt     int
}

func NewAudioStream(size int) *AudioStream {
	return &AudioStream{
		buffer: make([]byte, size),
	}
}

// Implementation of io.Reader.Read()
func (as *AudioStream) Read(p []byte) (int, error) {
	n := 0
	for i := range p {
		if as.stock == 0 {
			p[i] = 0
			as.fillZeroCnt++ // for debug
		} else {
			p[i] = as.buffer[as.r]
			as.r = (as.r + 1) % len(as.buffer)
			as.stock--
		}
		n++
		as.readCnt++ // for debug
	}
	return n, nil
}

// Used in APU
func (as *AudioStream) write(p []byte) int {
	n := 0
	for _, b := range p {
		if as.stock >= len(as.buffer) {
			as.r = (as.r + 1) % len(as.buffer)
			as.stock -= 1
			as.skipCnt += 1 // for debug
		}
		as.buffer[as.w] = b
		as.w = (as.w + 1) % len(as.buffer)
		as.stock++
		n++
		as.writeCnt++ // for debug
	}
	return n
}
