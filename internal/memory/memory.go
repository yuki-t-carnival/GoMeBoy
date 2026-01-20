package memory

import (
	"fmt"
	"gomeboy/internal/mbc"
)

type Memory struct {
	mbc  mbc.MBC
	wram [8][0x1000]byte // DMG=1bank, CGB=8bank
	hram [0x7F]byte
	io   [0x80]byte
	ie   byte

	wramBank byte // only CGB mode

	// Cartridge Header
	mbcType       int
	TotalROMBanks int
	TotalRAMBanks int
}

func NewMemory(rom, sav []byte) *Memory {
	mem := &Memory{}
	mbc.InitLists()
	mem.mbcType = mbc.MBCTypeList[rom[0x0147]]
	mem.TotalROMBanks = mbc.TotalROMBanksList[rom[0x0148]]
	mem.TotalRAMBanks = mbc.TotalRAMBanksList[rom[0x0149]]
	switch mem.mbcType {
	case 0:
		mem.mbc = mbc.NewMBC0(rom, sav, mem.TotalRAMBanks) // = No MBC
	case 1:
		mem.mbc = mbc.NewMBC1(rom, sav, mem.TotalRAMBanks)
	case 5:
		mem.mbc = mbc.NewMBC5(rom, sav, mem.TotalRAMBanks)
	default:
		panic("Unsupported MBC type")
	}
	return mem
}

// Called from Bus.Read()
func (m *Memory) Read(addr uint16) byte {
	switch {
	case addr < 0x8000:
		return m.mbc.ReadROM(addr)

	case addr >= 0x8000 && addr < 0xA000:
		return 0xFF // Access VRAM via Bus.Read()

	case addr >= 0xA000 && addr < 0xC000:
		return m.mbc.ReadERAM(addr)

	case addr >= 0xC000 && addr < 0xD000:
		return m.wram[0][addr-0xC000]
	case addr >= 0xD000 && addr < 0xE000:
		return m.wram[max(m.wramBank, 1)][addr-0xD000]

	case addr >= 0xE000 && addr < 0xFE00:
		mirror := addr - 0x2000
		return m.Read(mirror)

	case addr >= 0xFE00 && addr < 0xFEA0:
		return 0xFF // Access OAM via Bus.Read()

	case addr >= 0xFEA0 && addr < 0xFF00:
		return 0xFF

	case addr >= 0xFF00 && addr < 0xFF80:
		return m.io[addr-0xFF00]

	case addr >= 0xFF80 && addr < 0xFFFF:
		return m.hram[addr-0xFF80]

	case addr == 0xFFFF:
		return m.ie

	default:
		return 0xFF
	}
}

// Called from Bus.Write()
func (m *Memory) Write(addr uint16, val byte) {
	switch {
	case addr < 0x8000:
		m.mbc.WriteROM(addr, val) // Consider banking

	case addr >= 0x8000 && addr < 0xA000:
		return // Access VRAM via Bus.Write()

	case addr >= 0xA000 && addr < 0xC000:
		m.mbc.WriteERAM(addr, val)

	case addr >= 0xC000 && addr < 0xD000:
		m.wram[0][addr-0xC000] = val
	case addr >= 0xD000 && addr < 0xE000:
		m.wram[max(m.wramBank, 1)][addr-0xD000] = val

	case addr >= 0xE000 && addr < 0xFE00:
		mirror := addr - 0x2000
		m.Write(mirror, val)

	case addr >= 0xFE00 && addr < 0xFEA0:
		return // Access OAM via Bus.Write()

	case addr >= 0xFEA0 && addr < 0xFF00:
		return

	case addr >= 0xFF00 && addr < 0xFF80:
		m.io[addr-0xFF00] = val

	case addr >= 0xFF80 && addr < 0xFFFF:
		m.hram[addr-0xFF80] = val

	case addr == 0xFFFF:
		m.ie = val
	}
}

func (m *Memory) ReadWRAMBank() byte {
	return m.wramBank & 0x07
}

func (m *Memory) WriteWRAMBank(val byte) {
	m.wramBank = val & 0x07
}

func (m *Memory) GetHeaderInfo() []string {
	var mbc string
	switch m.mbcType {
	case -1:
		mbc = "Unsupported"
	case 0:
		mbc = "No MBC"
	default:
		mbc = fmt.Sprint(m.mbcType)
	}

	var rom string
	if m.TotalROMBanks == -1 {
		rom = "Unsupported"
	} else {
		rom = fmt.Sprintf("%d banks", m.TotalROMBanks)
	}

	var ram string
	if m.TotalRAMBanks == -1 {
		ram = "Unsupported"
	} else {
		ram = fmt.Sprintf("%d banks", m.TotalRAMBanks)
	}

	var strs []string
	strs = append(strs, "MBC:"+mbc)
	strs = append(strs, "ROM:"+rom)
	strs = append(strs, "RAM:"+ram)
	return strs
}

func (m *Memory) GetSaveData() []byte {
	return m.mbc.GetSaveData()
}
