// This file treats "No MBC" as "MBC0".

package mbc

type MBC0 struct {
	rom  []byte // =.gb data
	eram []byte // =External RAM, SRAM
}

func NewMBC0(rom, sav []byte, TotalRAMBanks int) *MBC0 {
	mbc0 := &MBC0{
		rom: rom,
	}

	mbc0.eram = make([]byte, TotalRAMBanks*0x2000)
	if len(sav) <= len(mbc0.eram) {
		copy(mbc0.eram, sav)
	}
	return mbc0
}

// Read from ROM in the current bank
func (mbc0 *MBC0) ReadROM(addr uint16) byte {
	switch {
	case addr < 0x8000:
		return mbc0.rom[uint32(addr)]
	default:
		return 0xFF
	}
}

// Read from eram in the current bank
func (mbc0 *MBC0) ReadERAM(addr uint16) byte {
	switch {
	case addr >= 0xA000 && addr < 0xC000:
		return mbc0.eram[uint32(addr)-0xA000]
	default:
		return 0xFF
	}
}

func (mbc0 *MBC0) WriteROM(addr uint16, val byte) {
}

// Write to eram area
func (mbc0 *MBC0) WriteERAM(addr uint16, val byte) {
	mbc0.eram[uint32(addr)-0xA000] = val
}

func (mbc0 *MBC0) GetSaveData() []byte {
	return mbc0.eram
}
