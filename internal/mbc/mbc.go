package mbc

type MBC interface {
	ReadROM(addr uint16) byte
	ReadERAM(addr uint16) byte
	WriteROM(addr uint16, val byte)
	WriteERAM(addr uint16, val byte)
	GetSaveData() []byte
}

var MBCTypeList [256]int
var TotalROMBanksList [256]int
var TotalRAMBanksList [256]int

func InitLists() {
	for i := range MBCTypeList {
		MBCTypeList[i] = -1
	}
	MBCTypeList[0x00] = 0 //"ROM ONLY"
	MBCTypeList[0x01] = 1 //"MBC1"
	MBCTypeList[0x02] = 1 //"MBC1+RAM"
	MBCTypeList[0x03] = 1 //"MBC1+RAM+BT"
	//MBCTypeList[0x05] = "MBC2"
	//MBCTypeList[0x06] = "MBC2+BT"
	//MBCTypeList[0x08] = "ROM+RAM 11"
	//MBCTypeList[0x09] = "ROM+RAM+BT 11"
	//MBCTypeList[0x0B] = "MMM01"
	//MBCTypeList[0x0C] = "MMM01+RAM"
	//MBCTypeList[0x0D] = "MMM01+RAM+BT"
	//MBCTypeList[0x0F] = "MBC3+T+BT"
	//MBCTypeList[0x10] = "MBC3+T+RAM+BT 12"
	//MBCTypeList[0x11] = "MBC3"
	//MBCTypeList[0x12] = "MBC3+RAM 12"
	//MBCTypeList[0x13] = "MBC3+RAM+BT 12"
	MBCTypeList[0x19] = 5 //"MBC5"
	MBCTypeList[0x1A] = 5 //"MBC5+RAM"
	MBCTypeList[0x1B] = 5 //"MBC5+RAM+BT"
	MBCTypeList[0x1C] = 5 //"MBC5+RBL"
	MBCTypeList[0x1D] = 5 //"MBC5+RBL+RAM"
	MBCTypeList[0x1E] = 5 //"MBC5+RBL+RAM+BT"
	//MBCTypeList[0x20] = "MBC6"
	//MBCTypeList[0x22] = "MBC7+SEN+RBL+RAM+BT"
	//MBCTypeList[0xFC] = "POCKET CAMERA"
	//MBCTypeList[0xFD] = "BANDAI TAMA5"
	//MBCTypeList[0xFE] = "HuC3"
	//MBCTypeList[0xFF] = "HuC1+RAM+BT"

	for i := range TotalROMBanksList {
		TotalROMBanksList[i] = -1
	}
	TotalROMBanksList[0x00] = 2   //"32KiB(2banksnoBanking)"
	TotalROMBanksList[0x01] = 4   //"64KiB(4banks)"
	TotalROMBanksList[0x02] = 8   //"128KiB(8banks)"
	TotalROMBanksList[0x03] = 16  //"256KiB(16banks)"
	TotalROMBanksList[0x04] = 32  //"512KiB(32banks)"
	TotalROMBanksList[0x05] = 64  //"1MiB(64banks)"
	TotalROMBanksList[0x06] = 128 //"2MiB(128banks)"
	TotalROMBanksList[0x07] = 256 //"4MiB(256banks)"
	TotalROMBanksList[0x08] = 512 //"8MiB(512banks)"
	TotalROMBanksList[0x52] = 72  //"1.1MiB(72banks)"
	TotalROMBanksList[0x53] = 80  //"1.2MiB(80banks)"
	TotalROMBanksList[0x54] = 96  //"1.5MiB(96banks)"

	for i := range TotalRAMBanksList {
		TotalRAMBanksList[i] = -1
	}
	TotalRAMBanksList[0x00] = 0  //"0(No RAM)"
	TotalRAMBanksList[0x01] = 0  //"-(Unused)"
	TotalRAMBanksList[0x02] = 1  //"8KiB(1bank)"
	TotalRAMBanksList[0x03] = 4  //"32KiB(4banks/8KiB)"
	TotalRAMBanksList[0x04] = 16 //"128KiB(16banks/8KiB)"
	TotalRAMBanksList[0x05] = 8  //"64KiB(8banks/8KiB)"
}
