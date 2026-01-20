package apu

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

const SampleRate = 44100
const CyclesPerSample = 4194304 / SampleRate
const SampPerLenTimerTick = SampleRate / 256
const SampPerEnvTick = SampleRate / 64

var b [4]byte // Declared here for optimization

type APU struct {
	AudioStream *AudioStream

	cycles float64

	// For debug
	debugStrs              []string
	timeOfDebugStrsCreated time.Time

	// Global control registers
	nr52 byte // Audio master control
	nr51 byte // Sound panning
	nr50 byte // Master volume & VIN panning

	// Sound channel 1
	nr10                  byte // Channel 1 sweep (not implemented)
	nr11                  byte // Channel 1 length timer & duty cycle
	nr12                  byte // Channel 1 volume & envelope
	nr13                  byte // Channel 1 period low [write-only]
	nr14                  byte // Channel 1 period high & control
	ch1Vol                byte
	ch1SampCntForLenTimer int
	ch1SampCntForEnv      int
	ch1LenTimer           int
	ch1Phase              float64

	// Sound channel 2
	nr21                  byte // Channel 2 length timer & duty cycle
	nr22                  byte // Channel 2 volume & envelope
	nr23                  byte // Channel 2 period low [write-only]
	nr24                  byte // Channel 2 period high & control
	ch2Vol                byte
	ch2SampCntForLenTimer int
	ch2SampCntForEnv      int
	ch2LenTimer           int
	ch2Phase              float64

	// Sound channel 3
	waveRAM               [16]byte
	nr30                  byte // Channel 3 DAC Enable
	nr31                  byte // Channel 3 length timer & duty cycle
	nr32                  byte // Channel 3 Output level
	nr33                  byte // Channel 3 period low [write-only]
	nr34                  byte // Channel 3 period high & control
	ch3SampCntForLenTimer int
	ch3LenTimer           int
	ch3Phase              float64
	idxWavRAM             int

	// Sound channel 4
	nr41                  byte // Channel 4 length timer [write-only]
	nr42                  byte // Channel 4 volume & envelope
	nr43                  byte // Channel 4 freqency & randomness
	nr44                  byte // Channel 4 control
	ch4Vol                byte
	ch4SampCntForLenTimer int
	ch4SampCntForEnv      int
	ch4SampCntForLFSR     int
	ch4LenTimer           int
	lfsr                  uint16
}

// ************************************ Public functions *******************************************

func NewAPU() *APU {
	a := &APU{
		AudioStream: NewAudioStream(SampleRate * 4),
		lfsr:        0x7FFF,
	}
	return a
}

func (a *APU) Step(cpuCycles int) {
	a.cycles += float64(cpuCycles)
	for a.cycles >= CyclesPerSample {
		a.cycles -= CyclesPerSample
		b := a.generateSample()
		a.AudioStream.write(b)
	}
}

func (a *APU) ReadWaveRAM(addr uint16) byte {
	return a.waveRAM[addr]
}

func (a *APU) WriteWaveRAM(addr uint16, val byte) {
	a.waveRAM[addr] = val
}

func (a *APU) GetAPUInfo() []string {
	if time.Since(a.timeOfDebugStrsCreated).Milliseconds() >= 500 {
		a.debugStrs = []string{}
		a.debugStrs = append(a.debugStrs, "BUF STATUS")
		a.debugStrs = append(a.debugStrs, fmt.Sprintf("Stock buf:%d", a.AudioStream.stock>>2))
		a.debugStrs = append(a.debugStrs, fmt.Sprintf("Fill0 cnt:%d", a.AudioStream.fillZeroCnt>>2))
		a.debugStrs = append(a.debugStrs, fmt.Sprintf("Skip cnt:%d", a.AudioStream.skipCnt>>2))
		a.debugStrs = append(a.debugStrs, fmt.Sprintf("Read cnt:%d", a.AudioStream.readCnt>>2))
		a.debugStrs = append(a.debugStrs, fmt.Sprintf("Write cnt:%d", a.AudioStream.writeCnt>>2))
		a.timeOfDebugStrsCreated = time.Now()
	}
	return a.debugStrs
}

// *********************************** Private functions *******************************************

// =================================== Generate Sample =============================================
func (a *APU) generateSample() []byte {
	sr1, sl1 := a.generateSquareChannel(1)
	sr2, sl2 := a.generateSquareChannel(2)
	sr3, sl3 := a.generateWaveChannel()
	sr4, sl4 := a.generateNoiseChannel()
	/* s1 = 0
	s2 = 0
	s4 = 0 */
	sr := (sr1 + sr2 + sr3 + sr4) / 4.0
	sl := (sl1 + sl2 + sl3 + sl4) / 4.0
	sampleR := int16(sr * 32767)
	sampleL := int16(sl * 32767)
	binary.LittleEndian.PutUint16(b[0:2], uint16(sampleL))
	binary.LittleEndian.PutUint16(b[2:4], uint16(sampleR))
	return b[:]
}

// ======================================= Channel 1/2 =============================================
func (a *APU) generateSquareChannel(ch byte) (float64, float64) {
	var nrX1 byte
	var nrX2 byte
	var nrX3 byte
	var nrX4 byte
	var chXPhase *float64
	var chXSampCntForLenTimer *int
	var chXLenTimer *int
	var chXSampCntForEnv *int
	var chXVol *byte
	switch ch {
	case 1:
		nrX1 = a.nr11
		nrX2 = a.nr12
		nrX3 = a.nr13
		nrX4 = a.nr14
		chXPhase = &a.ch1Phase
		chXSampCntForLenTimer = &a.ch1SampCntForLenTimer
		chXLenTimer = &a.ch1LenTimer
		chXSampCntForEnv = &a.ch1SampCntForEnv
		chXVol = &a.ch1Vol
	case 2:
		nrX1 = a.nr21
		nrX2 = a.nr22
		nrX3 = a.nr23
		nrX4 = a.nr24
		chXPhase = &a.ch2Phase
		chXSampCntForLenTimer = &a.ch2SampCntForLenTimer
		chXLenTimer = &a.ch2LenTimer
		chXSampCntForEnv = &a.ch2SampCntForEnv
		chXVol = &a.ch2Vol
	default:
		panic("")
	}

	period := (uint16(nrX4&0x07) << 8) | uint16(nrX3)
	freq := 131072.0 / (2048.0 - float64(period))
	*chXPhase += freq / float64(SampleRate)
	for *chXPhase >= 1.0 {
		*chXPhase -= 1.0
	}
	var dutyRatio float64
	switch nrX1 >> 6 { // Duty cycle value(binary)
	case 0:
		dutyRatio = 0.125
	case 1:
		dutyRatio = 0.25
	case 2:
		dutyRatio = 0.5
	case 3:
		dutyRatio = 0.75
	}

	a.execLengthTimer(ch, nrX4, chXLenTimer, chXSampCntForLenTimer)
	a.execEnvelope(chXVol, nrX2, chXSampCntForEnv)
	volR, volL := a.execMixing(ch, float64(*chXVol)/15.0)

	var sampleR, sampleL float64
	if *chXPhase < dutyRatio {
		sampleR = +volR
		sampleL = +volL
	} else {
		sampleR = -volR
		sampleL = -volL
	}
	return sampleR, sampleL
}

// ===================================== Channel 3 =================================================
func (a *APU) generateWaveChannel() (float64, float64) {
	period := (uint16(a.nr34&0x07) << 8) | uint16(a.nr33)
	freq := (65536.0 / (2048.0 - float64(period))) * 32

	a.ch3Phase += freq / float64(SampleRate)
	for a.ch3Phase >= 1.0 {
		a.ch3Phase -= 1.0
		a.idxWavRAM = (a.idxWavRAM + 1) % 32
	}

	// ram[0]hi, ram[0]lo, ram[1]hi...
	wav := a.waveRAM[a.idxWavRAM/2]
	if a.idxWavRAM%2 == 0 {
		wav >>= 4
	} else {
		wav &= 0x0F
	}

	// output level (=ch3 volume)
	switch (a.nr32 & 0x60) >> 5 {
	case 0:
		wav = 0
	case 2:
		wav >>= 1
	case 3:
		wav >>= 2
	}

	a.execLengthTimer(3, a.nr34, &a.ch3LenTimer, &a.ch3SampCntForLenTimer)
	volR, volL := a.execMixing(3, 1.0)

	if a.nr30&0x80 == 0 { // DAC on/off (ch3 only)
		volR = 0
		volL = 0
	}

	var sampleR, sampleL float64
	sampleR = (float64(wav)/7.5 - 1.0) * volR // ram[x]hi/lo = 0~15 to 0~2 to -1~+1
	sampleL = (float64(wav)/7.5 - 1.0) * volL // ram[x]hi/lo = 0~15 to 0~2 to -1~+1
	return sampleR, sampleL
}

// ===================================== Channel 4 =================================================
func (a *APU) generateNoiseChannel() (float64, float64) {
	clockShift := float64(a.nr43 >> 4)
	clockDivider := float64(a.nr43 & 0x03)
	if clockDivider == 0 {
		clockDivider = 0.5
	}
	lfsrWidth := (a.nr43 & (1 << 3)) >> 3 // 0: 15bit,   1: 7bit
	a.ch4SampCntForLFSR++
	// Set a min value for when LFSR clock > Sample rate
	samplesPerLFSRClock := max(1, int(SampleRate/(262144.0/(clockDivider*math.Pow(2, clockShift)))))
	for a.ch4SampCntForLFSR >= samplesPerLFSRClock {
		a.ch4SampCntForLFSR -= samplesPerLFSRClock
		xor := (a.lfsr & (1 << 0)) ^ ((a.lfsr & (1 << 1)) >> 1)
		a.lfsr = (xor << 15) | (a.lfsr & 0x7FFF)
		if lfsrWidth == 1 {
			a.lfsr = (xor << 7) | (a.lfsr & 0xFF7F)
		}
		a.lfsr >>= 1
	}

	a.execLengthTimer(4, a.nr44, &a.ch4LenTimer, &a.ch4SampCntForLenTimer)
	a.execEnvelope(&a.ch4Vol, a.nr42, &a.ch4SampCntForEnv)
	volR, volL := a.execMixing(4, float64(a.ch4Vol)/15.0)

	var sampleR, sampleL float64
	if a.lfsr&1 == 0 {
		sampleR = +volR
		sampleL = +volL
	} else {
		sampleR = -volR
		sampleL = -volL
	}
	return sampleR, sampleL
}

// =========================================== Mixing ==============================================
func (a *APU) execMixing(ch byte, chVol float64) (float64, float64) {
	panR := float64((a.nr51 & (1 << (ch - 1))) >> (ch - 1)) // right 0 or 1
	panL := float64((a.nr51 & (1 << (ch + 3))) >> (ch + 3)) // left 0 or 1
	masterR := float64(a.nr50&0x07) / 7.0
	masterL := float64((a.nr50&0x70)>>4) / 7.0
	volR := panR * masterR * chVol
	volL := panL * masterL * chVol
	// NR52: Audio Master Control
	if a.nr52&(1<<7) == 0 || a.nr52&(1<<(ch-1)) == 0 {
		volR = 0
		volL = 0
	}
	return volR, volL
}

// ======================================== Length Timer ===========================================
func (a *APU) execLengthTimer(ch, ctrlReg byte, lenTimer, sampCounter *int) {
	max := 64
	if ch == 3 {
		max = 256
	}
	if a.nr52&(1<<(ch-1)) != 0 && (ctrlReg&(1<<6) != 0) { // If chX is Enabled && Length timer is enabled
		(*sampCounter)++
		for *sampCounter >= int(SampPerLenTimerTick) { // Length timer tick up at 256Hz
			*sampCounter -= int(SampPerLenTimerTick)
			if *lenTimer == max { // If Length Timer is max value (CH1,2,4: 64,    CH3: 256)
				a.nr52 &^= (1 << (ch - 1)) // Disable chX
			} else {
				(*lenTimer)++
			}
		}
	}
}

// ========================================= Envelope ==============================================
func (a *APU) execEnvelope(chVol *byte, envReg byte, sampCounter *int) {
	envPeriod := envReg & 0x07
	if envPeriod != 0 {
		(*sampCounter)++
		for *sampCounter >= SampPerEnvTick*int(envPeriod) { // tick up
			*sampCounter -= SampPerEnvTick * int(envPeriod)
			isEnvUp := envReg&(1<<3) != 0
			if isEnvUp {
				if *chVol < 15 {
					(*chVol)++
				}
			} else { // Down
				if *chVol > 0 {
					(*chVol)--
				}
			}
		}
	}
}
