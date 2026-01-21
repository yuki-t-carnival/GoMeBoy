package emulator

import (
	"gomeboy/internal/bus"
	"gomeboy/internal/cpu"
	"gomeboy/internal/memory"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	CyclesPerFrame = 69905

	// Emulator mode ID
	RunMode   = 0
	PauseMode = 1
)

type Emulator struct {
	CPU *cpu.CPU

	IsPaused bool
	emuMode  int
	IsCGB    bool

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
		CPU:      c,
		emuMode:  RunMode,
		IsPaused: false,
	}

	cgbReg := e.CPU.Bus.Read(0x0143)
	if cgbReg == 0xC0 || cgbReg == 0x80 {
		e.IsCGB = true
		e.CPU.Bus.PPU.IsCGB = true
		e.CPU.Bus.PPU.SetOPRI(0xFE)
	}
	return e
}

// Run one Game Boy frame
func (e *Emulator) RunFrame() int {
	cycles := 0
	var cpuSpeed int
	isWspeed := e.CPU.Bus.PPU.IsCGB && e.CPU.Bus.IsWSpeed
	if isWspeed {
		cpuSpeed = 2
	} else {
		cpuSpeed = 1
	}

	for cycles < CyclesPerFrame*cpuSpeed {
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
		cycles += c
	}
	e.CPU.Bus.Joypad.Update()
	return 0
}

// KeyP: Toggle Run/Pause Mode
// KeyS: Run a single step
func (e *Emulator) updateEmuMode() {
	if e.isKeyP {
		if e.emuMode == RunMode {
			e.emuMode = PauseMode
		} else {
			e.emuMode = RunMode
		}
	}
	e.IsPaused = (e.emuMode == PauseMode) && !e.isKeyS
}

// In case of Panic, CPU status is output to the console
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
