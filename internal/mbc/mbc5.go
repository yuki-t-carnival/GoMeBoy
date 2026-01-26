package mbc

type MBC5 struct {
	rom           []byte // =.gb data
	eram          []byte // =External RAM, SRAM
	romBankLo     byte   // lower 8bit
	romBankHi     byte   // upper 1bit
	ramBank       byte
	isRAMEnable   bool
	totalRAMBanks int
}

func NewMBC5(rom, sav []byte, TotalRAMBanks int) *MBC5 {
	mbc5 := &MBC5{
		rom: rom,
	}
	mbc5.totalRAMBanks = TotalRAMBanks
	mbc5.eram = make([]byte, mbc5.totalRAMBanks*0x2000)
	if len(sav) <= len(mbc5.eram) {
		copy(mbc5.eram, sav)
	}
	return mbc5
}

// Read from ROM in the current bank
func (mbc5 *MBC5) ReadROM(addr uint16) byte {
	switch {
	case addr < 0x4000:
		return mbc5.rom[addr]

	case addr >= 0x4000 && addr < 0x8000: // ROM Bank 00 ~ 1FF
		bank := uint32(mbc5.romBankHi)<<8 | uint32(mbc5.romBankLo)
		return mbc5.rom[0x4000*bank+uint32(addr)-0x4000]
	default:
		return 0xFF
	}
}

// Read from eram in the current bank
func (mbc5 *MBC5) ReadERAM(addr uint16) byte {
	switch {
	case addr >= 0xA000 && addr < 0xC000:
		if !mbc5.isRAMEnable {
			return 0xFF
		}
		bank := mbc5.ramBank
		return mbc5.eram[uint32(bank)*0x2000+uint32(addr)-0xA000]
	default:
		return 0xFF
	}
}

// Write to ROM area
// (it is not a write to the ROM, but a write to the MBC register)
func (mbc5 *MBC5) WriteROM(addr uint16, val byte) {
	switch {
	case addr < 0x2000: // RAM Enable
		mbc5.isRAMEnable = val&0x0F == 0x0A

	// The 8 LSB of ROM bank number
	case addr >= 0x2000 && addr < 0x3000:
		mbc5.romBankLo = val

	// The 9th bit of ROM bank number
	case addr >= 0x3000 && addr < 0x4000:
		mbc5.romBankHi = val & 0x01

	// Banking Mode Select
	case addr >= 0x4000 && addr < 0x6000:
		mbc5.ramBank = val & 0x0F
	}
}

// Write to eram area
func (mbc5 *MBC5) WriteERAM(addr uint16, val byte) {
	switch {
	case addr >= 0xA000 && addr < 0xC000:
		if !mbc5.isRAMEnable {
			return
		}
		bank := mbc5.ramBank % byte(mbc5.totalRAMBanks)
		mbc5.eram[uint32(bank)*0x2000+uint32(addr)-0xA000] = val
	}
}

func (mbc5 *MBC5) GetSaveData() []byte {
	return mbc5.eram
}
