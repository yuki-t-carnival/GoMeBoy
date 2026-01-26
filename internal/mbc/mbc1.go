package mbc

type MBC1 struct {
	rom         []byte // =.gb data
	eram        []byte // =External RAM, SRAM
	bankingMode byte
	bankHigh    byte
	romBankLow  byte
	isRAMEnable bool
}

func NewMBC1(rom, sav []byte, TotalRAMBanks int) *MBC1 {
	mbc1 := &MBC1{
		rom:        rom,
		romBankLow: 1,
	}

	mbc1.eram = make([]byte, TotalRAMBanks*0x2000)
	if len(sav) <= len(mbc1.eram) {
		copy(mbc1.eram, sav)
	}
	return mbc1
}

// Read from ROM in the current bank
func (mbc1 *MBC1) ReadROM(addr uint16) byte {
	switch {
	case addr < 0x4000: // ROM Bank $20/$40/$60
		bank := byte(0)
		if mbc1.bankingMode == 1 {
			bank = mbc1.bankHigh << 5
		}
		return mbc1.rom[0x4000*uint32(bank)+uint32(addr)]

	case addr >= 0x4000 && addr < 0x8000: // ROM Bank 01-7F
		bank := (mbc1.bankHigh << 5) | mbc1.romBankLow
		return mbc1.rom[0x4000*uint32(bank)+uint32(addr-0x4000)]
	default:
		return 0xFF
	}
}

// Read from eram in the current bank
func (mbc1 *MBC1) ReadERAM(addr uint16) byte {
	switch {
	case addr >= 0xA000 && addr < 0xC000:
		if !mbc1.isRAMEnable {
			return 0xFF
		}
		bank := byte(0)
		if mbc1.bankingMode == 1 {
			bank = mbc1.bankHigh
		}
		return mbc1.eram[uint16(bank)*0x2000+addr-0xA000]
	default:
		return 0xFF
	}
}

// Write to ROM area
// (it is not a write to the ROM, but a write to the MBC register)
func (mbc1 *MBC1) WriteROM(addr uint16, val byte) {
	switch {
	case addr < 0x2000: // RAM Enable
		mbc1.isRAMEnable = val&0x0F == 0x0A

	// ROM Bank Number
	case addr >= 0x2000 && addr < 0x4000:
		mbc1.romBankLow = val & 0x1F
		if mbc1.romBankLow == 0 {
			mbc1.romBankLow = 1
		}

	// RAM Bank Number or Upper Bits of ROM Bank Number
	case addr >= 0x4000 && addr < 0x6000:
		mbc1.bankHigh = val & 0x03

	// Banking Mode Select
	case addr >= 0x6000 && addr < 0x8000:
		mbc1.bankingMode = val & 0x01
	}
}

// Write to eram area
func (mbc1 *MBC1) WriteERAM(addr uint16, val byte) {
	switch {
	case addr >= 0xA000 && addr < 0xC000:
		if !mbc1.isRAMEnable {
			return
		}
		bank := byte(0)
		if mbc1.bankingMode == 1 {
			bank = mbc1.bankHigh
		}
		mbc1.eram[uint16(bank)*0x2000+addr-0xA000] = val
	}
}

func (mbc1 *MBC1) GetSaveData() []byte {
	return mbc1.eram
}
