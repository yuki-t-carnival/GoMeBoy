package emulator

import (
	"gomeboy/internal/bus"
	"gomeboy/internal/cpu"
	"gomeboy/internal/memory"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	CyclesPerFrame float64 = 4194304.0 / 60.0
)

type Emulator struct {
	CPU       *cpu.CPU
	cpuCycles float64

	IsPaused    bool
	IsPauseMode bool
	IsCGB       bool
	ROMTitle    string

	isKeyP       bool
	isKeyS       bool
	isKeyEsc     bool
	isPrevKeyP   bool
	isPrevKeyS   bool
	isPrevKeyEsc bool
}

func NewEmulator(rom, sav []byte) *Emulator {
	m := memory.NewMemory(rom, sav)
	b := bus.NewBus(m)
	c := cpu.NewCPU(b)
	c.Tracer = cpu.NewTracer(c)

	e := &Emulator{
		CPU:         c,
		IsPauseMode: false,
		IsPaused:    false,
	}

	e.ROMTitle = e.GetROMTitle(rom)

	cgbReg := e.CPU.Bus.Read(0x0143)
	if cgbReg == 0xC0 || cgbReg == 0x80 {
		e.IsCGB = true
		e.CPU.Bus.PPU.IsCGB = true
		e.CPU.Bus.PPU.SetOPRI(0xFE)
	}
	return e
}

func (e *Emulator) RunFrame() int {
	cpuSpeed := 1
	if e.CPU.Bus.PPU.IsCGB && e.CPU.Bus.IsWSpeed {
		cpuSpeed = 2
	}
	maxCycles := CyclesPerFrame * float64(cpuSpeed)
	e.CPU.Bus.Joypad.Update()
	for e.cpuCycles < maxCycles {
		e.updateEbitenKeys()
		e.updateEmuMode()
		if e.CPU.IsPanic || e.isKeyEsc { // for debug
			e.panicDump()
			return -1
		} else if e.IsPaused {
			return 0
		}
		var c int
		c = e.CPU.Step()
		e.CPU.Bus.Timer.Step(c, e.CPU.IsStopped)
		e.CPU.Bus.PPU.Step(c / cpuSpeed)
		e.CPU.Bus.APU.Step(c / cpuSpeed)
		e.CPU.Tracer.Record(e.CPU)
		e.cpuCycles += float64(c)
	}
	e.cpuCycles -= maxCycles
	return 0
}

// KeyP: Toggle Run/Pause Mode
// KeyS: Run a single step
func (e *Emulator) updateEmuMode() {
	if e.isKeyP {
		e.IsPauseMode = !e.IsPauseMode
	}
	e.IsPaused = e.IsPauseMode && !e.isKeyS
}

// In case of Panic, CPU status is output to the console.
func (e *Emulator) panicDump() {
	e.CPU.Tracer.Dump()
}

func (e *Emulator) updateEbitenKeys() {
	isP := ebiten.IsKeyPressed(ebiten.KeyP)
	isS := ebiten.IsKeyPressed(ebiten.KeyS)
	isEsc := ebiten.IsKeyPressed(ebiten.KeyEscape)
	e.isKeyP = !e.isPrevKeyP && isP
	e.isKeyS = !e.isPrevKeyS && isS
	e.isKeyEsc = !e.isPrevKeyEsc && isEsc
	e.isPrevKeyP = isP
	e.isPrevKeyS = isS
	e.isPrevKeyEsc = isEsc
}

func (e *Emulator) GetROMTitle(rom []byte) string {
	s := string(rom[0x0134:0x0143])
	firstNullIdx := strings.IndexByte(s, 0)
	if firstNullIdx != -1 {
		s = s[:firstNullIdx]
	}
	return s
}

func (e *Emulator) GetDebugLog() []string {
	var state string
	if e.IsPaused {
		state = "PAUSED"
	} else {
		state = "PLAY  "
	}
	strs := []string{}
	strs = append(strs, state)
	strs = append(strs, "")
	strs = append(strs, e.CPU.Tracer.GetCPUInfo()...)
	strs = append(strs, "")
	strs = append(strs, e.CPU.Bus.Memory.GetHeaderInfo()...)
	strs = append(strs, "")
	strs = append(strs, e.CPU.Bus.APU.GetAPUInfo()...)
	return strs
}
