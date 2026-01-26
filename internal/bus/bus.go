package bus

import (
	"gomeboy/internal/apu"
	"gomeboy/internal/joypad"
	"gomeboy/internal/memory"
	"gomeboy/internal/ppu"
	"gomeboy/internal/timer"
)

type Bus struct {
	PPU    *ppu.PPU
	Timer  *timer.Timer
	Joypad *joypad.Joypad
	Memory *memory.Memory
	APU    *apu.APU

	// DMA Transfer
	IsDMATransferInProgress bool
	DMATransferIndex        int // 0 ~ 159

	// CGB double speed mode (KEY1/SPD)
	IsWSpeed      bool
	IsSwitchArmed bool
}

const (
	// I/O Registers (FF00 ~ FF7F)
	P1_JOYP      uint16 = 0xFF00
	SB           uint16 = 0xFF01
	SC           uint16 = 0xFF02
	DIV          uint16 = 0xFF04
	TIMA         uint16 = 0xFF05
	TMA          uint16 = 0xFF06
	TAC          uint16 = 0xFF07
	IF           uint16 = 0xFF0F
	NR10         uint16 = 0xFF10
	NR11         uint16 = 0xFF11
	NR12         uint16 = 0xFF12
	NR13         uint16 = 0xFF13
	NR14         uint16 = 0xFF14
	NR21         uint16 = 0xFF16
	NR22         uint16 = 0xFF17
	NR23         uint16 = 0xFF18
	NR24         uint16 = 0xFF19
	NR30         uint16 = 0xFF1A
	NR31         uint16 = 0xFF1B
	NR32         uint16 = 0xFF1C
	NR33         uint16 = 0xFF1D
	NR34         uint16 = 0xFF1E
	NR41         uint16 = 0xFF20
	NR42         uint16 = 0xFF21
	NR43         uint16 = 0xFF22
	NR44         uint16 = 0xFF23
	NR50         uint16 = 0xFF24
	NR51         uint16 = 0xFF25
	NR52         uint16 = 0xFF26
	WaveRAMStart uint16 = 0xFF30
	LCDC         uint16 = 0xFF40
	STAT         uint16 = 0xFF41
	SCY          uint16 = 0xFF42
	SCX          uint16 = 0xFF43
	LY           uint16 = 0xFF44
	LYC          uint16 = 0xFF45
	DMA          uint16 = 0xFF46
	BGP          uint16 = 0xFF47
	OBP0         uint16 = 0xFF48
	OBP1         uint16 = 0xFF49
	WY           uint16 = 0xFF4A
	WX           uint16 = 0xFF4B
	KEY0_SYS     uint16 = 0xFF4C
	KEY1_SPD     uint16 = 0xFF4D
	VBK          uint16 = 0xFF4F
	BANK         uint16 = 0xFF50
	HDMA1        uint16 = 0xFF51
	HDMA2        uint16 = 0xFF52
	HDMA3        uint16 = 0xFF53
	HDMA4        uint16 = 0xFF54
	HDMA5        uint16 = 0xFF55
	RP           uint16 = 0xFF56
	BCPS_BGPI    uint16 = 0xFF68
	BCPD_BGPD    uint16 = 0xFF69
	OCPS_OBPI    uint16 = 0xFF6A
	OCPD_OBPD    uint16 = 0xFF6B
	OPRI         uint16 = 0xFF6C
	SVBK_WBK     uint16 = 0xFF70
	PCM12        uint16 = 0xFF76
	PCM34        uint16 = 0xFF77

	// Interrupt Enable Register
	IE uint16 = 0xFFFF
)

func NewBus(m *memory.Memory) *Bus {
	bus := &Bus{
		PPU:   ppu.NewPPU(),
		APU:   apu.NewAPU(),
		Timer: timer.NewTimer(),

		Joypad: joypad.NewJoypad(),
		Memory: m,
	}
	// Serial
	bus.Write(SB, 0x00)
	bus.Write(SC, 0x7E)

	// Interrupt
	bus.Write(IF, 0x01)
	bus.Write(IE, 0x00)

	// Sound
	bus.Write(NR10, 0x80)
	bus.Write(NR11, 0xBF)
	bus.Write(NR12, 0xF3)
	bus.Write(NR14, 0xBF)
	bus.Write(NR21, 0x3F)
	bus.Write(NR22, 0x00)
	bus.Write(NR24, 0xBF)
	bus.Write(NR30, 0x7F)
	bus.Write(NR31, 0xFF)
	bus.Write(NR32, 0x9F)
	bus.Write(NR34, 0xBF)
	bus.Write(NR41, 0xFF)
	bus.Write(NR42, 0x00)
	bus.Write(NR43, 0x00)
	bus.Write(NR44, 0xBF)
	bus.Write(NR50, 0x77)
	bus.Write(NR51, 0xF3)
	bus.Write(NR52, 0xF1)

	return bus
}

// The Bus.Read accesses the I/O, VRAM, OAM,
// and request other accesses to Memory.
func (b *Bus) Read(addr uint16) byte {
	switch {
	// PPU
	case addr >= 0x8000 && addr < 0xA000:
		return b.PPU.ReadVRAM(addr - 0x8000)
	case addr >= 0xFE00 && addr < 0xFEA0:
		return b.PPU.ReadOAM(addr - 0xFE00)
	case addr == VBK:
		return b.PPU.GetVBK()
	case addr == DMA:
		return b.PPU.GetDMA()
	case addr == LCDC:
		return b.PPU.GetLCDC()
	case addr == STAT:
		return b.PPU.GetSTAT()
	case addr == LY:
		return b.PPU.GetLY()
	case addr == LYC:
		return b.PPU.GetLYC()
	case addr == OBP0:
		return b.PPU.GetOBP0()
	case addr == OBP1:
		return b.PPU.GetOBP1()
	case addr == BGP:
		return b.PPU.GetBGP()
	case addr == WY:
		return b.PPU.GetWY()
	case addr == WX:
		return b.PPU.GetWX()
	case addr == SCY:
		return b.PPU.GetSCY()
	case addr == SCX:
		return b.PPU.GetSCX()
	case addr == BCPS_BGPI:
		return b.PPU.GetBCPS()
	case addr == BCPD_BGPD:
		return b.PPU.GetBCPD()
	case addr == OCPS_OBPI:
		return b.PPU.GetOCPS()
	case addr == OCPD_OBPD:
		return b.PPU.GetOCPD()
	case addr == HDMA5:
		return b.PPU.GetHDMA5()
	case addr == OPRI:
		return b.PPU.GetOPRI()

	// Timer
	case addr == DIV:
		return b.Timer.GetDIV()
	case addr == TIMA:
		return b.Timer.GetTIMA()
	case addr == TMA:
		return b.Timer.GetTMA()
	case addr == TAC:
		return b.Timer.GetTAC()

	// Joypad
	case addr == P1_JOYP:
		return b.Joypad.GetP1JOYP()

	// Interrupt
	case addr == IF:
		return b.Memory.Read(addr) | 0xE0
	case addr == IE:
		return b.Memory.Read(addr) | 0xE0

	// CPU
	case addr == KEY1_SPD:
		return b.GetKEY1()

	// APU
	case addr == NR10:
		return b.APU.GetNR10()
	case addr == NR11:
		return b.APU.GetNR11()
	case addr == NR12:
		return b.APU.GetNR12()
	case addr == NR13:
		return b.APU.GetNR13()
	case addr == NR14:
		return b.APU.GetNR14()
	case addr == NR21:
		return b.APU.GetNR21()
	case addr == NR22:
		return b.APU.GetNR22()
	case addr == NR23:
		return b.APU.GetNR23()
	case addr == NR24:
		return b.APU.GetNR24()
	case addr == NR30:
		return b.APU.GetNR30()
	case addr == NR31:
		return b.APU.GetNR31()
	case addr == NR32:
		return b.APU.GetNR32()
	case addr == NR33:
		return b.APU.GetNR33()
	case addr == NR34:
		return b.APU.GetNR34()
	case addr == NR41:
		return b.APU.GetNR41()
	case addr == NR42:
		return b.APU.GetNR42()
	case addr == NR43:
		return b.APU.GetNR43()
	case addr == NR44:
		return b.APU.GetNR44()
	case addr == NR50:
		return b.APU.GetNR50()
	case addr == NR51:
		return b.APU.GetNR51()
	case addr == NR52:
		return b.APU.GetNR52()
	case addr >= WaveRAMStart && addr < WaveRAMStart+16:
		return b.APU.ReadWaveRAM(addr - WaveRAMStart)

	// Memory
	case addr == SVBK_WBK:
		return b.Memory.ReadWRAMBank()
	default:
		return b.Memory.Read(addr)
	}
}

// The Bus.Write accesses the I/O, VRAM, OAM,
// and request other accesses to Memory.
func (b *Bus) Write(addr uint16, val byte) {
	switch {
	// PPU
	case addr >= 0x8000 && addr < 0xA000:
		b.PPU.WriteVRAM(addr-0x8000, val)
	case addr >= 0xFE00 && addr < 0xFEA0:
		b.PPU.WriteOAM(addr-0xFE00, val)
	case addr == VBK:
		b.PPU.SetVBK(val)
	case addr == DMA:
		b.PPU.SetDMA(val)
		if val <= 0xDF {
			b.IsDMATransferInProgress = true
			b.DMATransferIndex = 0
		}
	case addr == LCDC:
		b.PPU.SetLCDC(val) // TODO: LCD&PPU can be disabled only during VBlank period.
	case addr == STAT:
		b.PPU.SetSTAT(val)
	case addr == SCY:
		b.PPU.SetSCY(val)
	case addr == SCX:
		b.PPU.SetSCX(val)
	case addr == LY:
		// Writing is prohibited.
	case addr == LYC:
		b.PPU.SetLYC(val)
	case addr == BGP:
		b.PPU.SetBGP(val)
	case addr == OBP0:
		b.PPU.SetOBP0(val)
	case addr == OBP1:
		b.PPU.SetOBP1(val)
	case addr == WY:
		b.PPU.SetWY(val)
	case addr == WX:
		b.PPU.SetWX(val)
	case addr == BCPS_BGPI:
		b.PPU.SetBCPS(val)
	case addr == BCPD_BGPD:
		b.PPU.SetBCPD(val)
	case addr == OCPS_OBPI:
		b.PPU.SetOCPS(val)
	case addr == OCPD_OBPD:
		b.PPU.SetOCPD(val)
	case addr == HDMA1:
		b.PPU.SetHDMA1(val)
	case addr == HDMA2:
		b.PPU.SetHDMA2(val)
	case addr == HDMA3:
		b.PPU.SetHDMA3(val)
	case addr == HDMA4:
		b.PPU.SetHDMA4(val)
	case addr == HDMA5:
		b.PPU.SetHDMA5(val)
		b.vdmaTransfer()
	case addr == OPRI:
		b.PPU.SetOPRI(val)

	// Timer
	case addr == DIV:
		b.Timer.ResetDiv()
	case addr == TIMA:
		b.Timer.SetTIMA(val)
	case addr == TMA:
		b.Timer.SetTMA(val)
	case addr == TAC:
		b.Timer.SetTAC(val)

	// Joypad
	case addr == P1_JOYP:
		b.Joypad.SetP1JOYP(val)

	// Interrupt
	case addr == IF:
		b.Memory.Write(IF, val&0x1F)
	case addr == IE:
		b.Memory.Write(IE, val&0x1F)

	// CPU
	case addr == KEY1_SPD:
		b.SetKEY1(val)

	// APU
	case addr == NR10:
		b.APU.SetNR10(val)
	case addr == NR11:
		b.APU.SetNR11(val)
	case addr == NR12:
		b.APU.SetNR12(val)
	case addr == NR13:
		b.APU.SetNR13(val)
	case addr == NR14:
		b.APU.SetNR14(val)
	case addr == NR21:
		b.APU.SetNR21(val)
	case addr == NR22:
		b.APU.SetNR22(val)
	case addr == NR23:
		b.APU.SetNR23(val)
	case addr == NR24:
		b.APU.SetNR24(val)
	case addr == NR30:
		b.APU.SetNR30(val)
	case addr == NR31:
		b.APU.SetNR31(val)
	case addr == NR32:
		b.APU.SetNR32(val)
	case addr == NR33:
		b.APU.SetNR33(val)
	case addr == NR34:
		b.APU.SetNR34(val)
	case addr == NR41:
		b.APU.SetNR41(val)
	case addr == NR42:
		b.APU.SetNR42(val)
	case addr == NR43:
		b.APU.SetNR43(val)
	case addr == NR44:
		b.APU.SetNR44(val)
	case addr == NR50:
		b.APU.SetNR50(val)
	case addr == NR51:
		b.APU.SetNR51(val)
	case addr == NR52:
		b.APU.SetNR52(val)
	case addr >= WaveRAMStart && addr < WaveRAMStart+16:
		b.APU.WriteWaveRAM(addr-WaveRAMStart, val)

	// Memory
	case addr == SVBK_WBK:
		b.Memory.WriteWRAMBank(val)
	default:
		b.Memory.Write(addr, val)
	}
}

func (b *Bus) DMATransfer() {
	if !(b.PPU.GetDMA() <= 0xDF) || !b.IsDMATransferInProgress {
		panic("DMA transfer error")
	}
	srcBase := uint16(b.PPU.GetDMA()) << 8
	i := uint16(b.DMATransferIndex)
	v := b.Read(srcBase + i)
	b.PPU.WriteOAM(i, v)
	b.DMATransferIndex++
	if b.DMATransferIndex == 160 {
		b.IsDMATransferInProgress = false
		b.DMATransferIndex = 0
		return
	}
}

func (b *Bus) vdmaTransfer() {
	src := b.PPU.VDMASrc
	dst := b.PPU.VDMADst
	len := b.PPU.VDMALen
	for i := 0; i < len; i++ {
		b.PPU.WriteVRAM(src+uint16(i), b.Read(dst+uint16(i)))
	}
	b.PPU.VDMALen = 0
}

func (b *Bus) GetKEY1() byte {
	v := byte(0x7E)
	if b.IsWSpeed {
		v |= 0x80
	}
	if b.IsSwitchArmed {
		v |= 0x01
	}
	return v
}

func (b *Bus) SetKEY1(val byte) {
	b.IsWSpeed = val&0x80 != 0
	b.IsSwitchArmed = val&0x01 != 0
}
