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
	fillZeroCounter := 0
	n := 0
	for i := range p {
		if as.stock == 0 {
			p[i] = 0
			as.fillZeroCnt++ // for debug
			fillZeroCounter++
		} else {
			p[i] = as.buffer[as.r]
			as.r = (as.r + 1) % len(as.buffer)
			as.stock--
		}
		n++
	}
	mod := fillZeroCounter % 4
	as.r = (as.r + mod + len(as.buffer)) % len(as.buffer)
	as.stock += mod
	return n, nil
}

// Used in APU
func (as *AudioStream) write(p [4]byte) {
	if as.stock > len(as.buffer)-4 {
		wasteAmount := len(as.buffer) >> 3 << 2
		as.r = (as.r + wasteAmount) % len(as.buffer)
		as.stock -= wasteAmount
		as.skipCnt += wasteAmount
	}
	for _, b := range p {
		as.buffer[as.w] = b
		as.w = (as.w + 1) % len(as.buffer)
		as.stock++
	}
}
