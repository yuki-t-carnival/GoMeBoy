package ppu

import (
	"gomeboy/internal/util"
	"image"
	"image/color"
	"math"
)

const (
	DMG_OBP0 = 0
	DMG_OBP1 = 1
	DMG_BGP  = 2

	CGB_OBP0 = 0
	CGB_BGP0 = 8

	Background = 0
	Window     = 1
)

var xFlipLUT [256]byte
var expand5bitLUT [32]byte

type PPU struct {
	pixelClues [160 * 144]PixelClues
	screen     *image.RGBA

	vram [2][0x2000]byte
	oam  [160]byte

	// I/O Registers (direct)
	lcdc byte
	dma  byte
	stat byte
	ly   byte
	lyc  byte
	wy   byte
	wx   byte
	scy  byte
	scx  byte
	obp0 byte // DMG mode only
	obp1 byte // DMG mode only
	bgp  byte // DMG mode only
	bcps byte // CGB mode only
	bcpd byte // CGB mode only
	ocps byte // CGB mode only
	ocpd byte // CGB mode only
	opri byte // CGB mode only
	vbk  byte // CGB mode only

	// Prev Registers
	isPrevLCDC byte

	drawingObjectList []int

	// PPU Internal Counters
	wly  int
	dots int

	// Interrupt
	HasLCDInterruptRequested    bool
	HasVBlankInterruptRequested bool

	bgpRAM           [64]byte // CGB mode only
	obpRAM           [64]byte // CGB mode only
	dmgRGBAColorList [4]color.RGBA

	IsCGB bool

	// HDMA
	VDMASrc uint16 // CGB mode only
	VDMALen int    // CGB mode only
	VDMADst uint16 // CGB mode only
}

type PixelClues struct {
	colorIndex               byte // ColorIndex (0 ~ 3)
	palette                  int  // BGP, PlaletteOBP0, OBP1
	isCGBBGMapPriorityBitSet bool //
}

func NewPPU() *PPU {
	calcXFlipLUT()
	calcExpand5bitLUT()
	p := &PPU{
		stat:   0x85,
		ly:     0x00,
		lyc:    0x00,
		wy:     0x00,
		wx:     0x00,
		scy:    0x00,
		scx:    0x00,
		screen: image.NewRGBA(image.Rect(0, 0, 160, 144)),
		dmgRGBAColorList: [4]color.RGBA{
			{255, 255, 128, 255},
			{160, 192, 64, 255},
			{64, 128, 64, 255},
			{0, 24, 0, 255},
		},
	}
	p.SetLCDC(0x91)
	p.SetBGP(0xFC)
	p.SetOBP0(0xFF)
	p.SetOBP1(0xFF)
	return p
}

func calcXFlipLUT() {
	result := byte(0)
	for i := 0; i < 256; i++ {
		result = 0
		for b := 0; b < 8; b++ {
			if i&(1<<b) != 0 {
				result |= 1 << (7 - b)
			}
		}
		xFlipLUT[i] = result
	}
}

func calcExpand5bitLUT() {
	maxI := 31.0
	maxO := 255.0

	for i := 0; i < 32; i++ {
		fi := float64(i)

		vivid := fi * (maxO / maxI)

		maxJ := maxO * maxO
		j := fi * (maxJ / maxI)
		pastel := math.Sqrt(j)

		expand5bitLUT[i] = byte((vivid + pastel) / 2)
	}
}

func (p *PPU) Step(cpuCycles int) {

	// 1 Line == 456 CPU cycles
	p.dots += cpuCycles
	if p.dots >= 456 {
		p.dots -= 456
		p.ly++
		if p.ly == 154 {
			p.ly = 0
		}

		// In case of PPU disabled
		if p.lcdc&(1<<7) == 0 {
			p.ly = 0
			p.wly = 0
			return
		}

		// Check LYC == LY Interrupts.
		p.stat = p.stat &^ (1 << 2) // LYC == LY bit clear
		if p.lyc == p.ly {
			p.stat |= 1 << 2
			if p.stat&(1<<6) != 0 { // Is LYC interrupt selected?
				p.HasLCDInterruptRequested = true
			}
		}

		switch {
		// During the period LY = 0 ~ 143, repeat Mode2, Mode3, Mode0, Mode2... every LY.
		case p.ly <= 143:
			// The cartridge program may set the drawing options to LCDC etc. later than dot = 0,
			// so the start of the drawing process is delayed a little.
			/* if p.hasTransferRq && p.dots >= 280 { */
			p.setPPUMode(2)
			p.oamSearch()
			p.setPPUMode(3)
			p.pixelTransfer()
			p.setPPUMode(0)
		// VBlank during LY = 144 ~ 153 (VBlank IRQ occurs only once at the moment LY = 144 is reached).
		case p.ly == 144:
			p.setPPUMode(1)
			p.HasVBlankInterruptRequested = true
			p.wly = 0
			p.drawGameBoyScreen()
		}
		p.checkSTATInt()
	}
}

// Update the frame buffer by one line
func (p *PPU) pixelTransfer() {

	isBGAndWindowEnableBitSet := p.lcdc&(1<<0) != 0
	if isBGAndWindowEnableBitSet || p.IsCGB {
		p.MapTransfer(Background)

		isWindowEnableBitSet := p.lcdc&(1<<5) != 0
		if isWindowEnableBitSet {

			isPrevWindowEnableBitSet := p.isPrevLCDC&(1<<5) != 0
			if !isPrevWindowEnableBitSet {
				p.wly = 0
			}
			if p.ly >= p.wy {
				isWindowDrawn := p.MapTransfer(Window)
				if isWindowDrawn {
					p.wly++
				}
			}
		}
		p.isPrevLCDC = p.lcdc
	}
	isOBJEnableBitSet := p.lcdc&(1<<1) != 0
	if isOBJEnableBitSet {
		p.objectsTransfer()
	}
}

func (p *PPU) setPPUMode(nextMode int) {
	p.stat = p.stat&0x7C | (byte(nextMode) & 0x03)
}

func (p *PPU) checkSTATInt() {
	mode := p.stat & 0x03

	isModeXIntSelectBitSet := p.stat&(1<<(mode+3)) != 0
	if isModeXIntSelectBitSet {
		p.HasLCDInterruptRequested = true
	}
}

// The oamSearch lists up to 10 objects to be displayed on the current scanline,
// sorts them in drawing order, and saves them in PPU.drawingObjectList.
func (p *PPU) oamSearch() {
	ly := int(p.ly)
	objectHeight := 8
	if p.lcdc&(1<<2) != 0 {
		objectHeight = 16
	}
	unsortedList := []int{}
	for i := 0; i < 40; i++ {
		objectY0 := int(p.oam[i<<2+0]) - 16
		areOverlapping := objectY0 <= ly && ly < objectY0+objectHeight
		if areOverlapping {
			unsortedList = append(unsortedList, i)
			if len(unsortedList) == 10 {
				break
			}
		}
	}

	// Sort the list "X-position descendig" or "OAM index descending".
	p.drawingObjectList = p.drawingObjectList[:0]
	isCGBStylePrioritySelected := p.opri&0x01 == 0
	isListOAMDescending := p.IsCGB && isCGBStylePrioritySelected
	if isListOAMDescending {
		for _, v := range unsortedList {
			p.drawingObjectList = util.InsertSlice(p.drawingObjectList, 0, v)
		}
	} else { // If not, the list is ordered by x-position descending.
		for _, oami := range unsortedList {
			insertPosition := len(p.drawingObjectList)
			for j, oamj := range p.drawingObjectList {
				if p.oam[oami<<2+1] >= p.oam[oamj<<2+1] {
					insertPosition = j
					break
				}
			}
			p.drawingObjectList = util.InsertSlice(p.drawingObjectList, insertPosition, oami)
		}
	}
}

// Draw the current line objects listed by oamSearch() to pixels
func (p *PPU) objectsTransfer() {

	ly := int(p.ly)

	// Draw objects to pixels in the order sorted above
	for i := range p.drawingObjectList {
		// Get Object attribytes in the OAM
		baseAdress := uint16(p.drawingObjectList[i] << 2)
		objectY0 := int(p.oam[baseAdress]) - 16
		objectX0 := int(p.oam[baseAdress+1]) - 8
		tileIndex := int(p.oam[baseAdress+2])
		objectAttributes := p.oam[baseAdress+3]

		tileData := [2]byte{}
		tileData = p.getObjectTile(tileIndex, objectAttributes, ly-objectY0)

		var palette int
		if p.IsCGB {
			cgbPalette := int(objectAttributes & 0x07)
			palette = CGB_OBP0 + cgbPalette
		} else {
			dmgPalette := int(objectAttributes & (1 << 4) >> 4)
			palette = DMG_OBP0 + dmgPalette
		}

		// Draw tileData(one row) on pixels
		targetY := ly * 160
		for b := 0; b < 8; b++ {
			targetX := objectX0 + b
			if targetX < 0 || targetX >= 160 {
				continue
			}
			target := targetY + targetX

			bgOrWindowColorIndex := p.pixelClues[target].colorIndex
			isBGAndWindowPriorityEnabled := p.lcdc&(1<<0) != 0

			if isBGAndWindowPriorityEnabled {
				if p.IsCGB {
					if p.pixelClues[target].isCGBBGMapPriorityBitSet &&
						bgOrWindowColorIndex != 0 {
						continue
					}
				}
				isObjectAttributesPriorityBitSet := objectAttributes&(1<<7) != 0
				if isObjectAttributesPriorityBitSet &&
					bgOrWindowColorIndex != 0 {
					continue
				}
			}

			lo := tileData[0] >> (7 - b) & 1
			hi := tileData[1] >> (7 - b) & 1
			colorIndex := hi<<1 | lo

			if colorIndex == 0 {
				continue
			}

			p.pixelClues[target].palette = palette
			p.pixelClues[target].colorIndex = colorIndex
		}
	}
}

// Get one row of tileData as an object
func (p *PPU) getObjectTile(tileIndex int, objectAttributes byte, tilePixelY int) [2]byte {
	isYFlip := objectAttributes&(1<<6) != 0
	isXFlip := objectAttributes&(1<<5) != 0
	bank := int(0)
	if p.IsCGB && objectAttributes&(1<<3) != 0 {
		bank = 1
	}
	if p.lcdc&(1<<2) != 0 { // OBJ size
		tileIndex &= 0xFE // When 8x16, index is masked to even colorIndexbers only
		if isYFlip {
			tilePixelY = 15 - tilePixelY
		}
	} else {
		if isYFlip {
			tilePixelY = 7 - tilePixelY
		}
	}
	base := tileIndex << 4
	data := [2]byte{}
	for i := 0; i < 2; i++ {
		addr := base + tilePixelY<<1 + i
		data[i] = p.vram[bank][addr]
		if isXFlip {
			data[i] = xFlipLUT[data[i]]
		}
	}
	return data
}

// Draw the current line background to pixels
func (p *PPU) MapTransfer(mapType int) bool {
	isWindowDrawn := false

	var data [2]byte
	pixelsIndexY := int(p.ly) * 160

	var mapPixelY int
	if mapType == Background {
		mapPixelY = int(p.ly + p.scy)
	} else {
		mapPixelY = int(p.wly)
	}

	mapRow := mapPixelY / 8
	tilePixelY := mapPixelY % 8
	palette := DMG_BGP // If DMG, Always BGP
	var isXFlip bool
	var isCGBBGMapPriorityBitSet bool

	for x := 0; x < 160; x++ {

		var mapPixelX int
		if mapType == Background {
			mapPixelX = int(byte(x) + p.scx) // wrapped
		} else {
			mapPixelX = x - (int(p.wx) - 7)
		}

		tilePixelX := mapPixelX % 8

		// Get tileData only when needed
		if x == 0 || tilePixelX == 0 {
			mapCol := mapPixelX / 8
			mapIndex := mapRow*32 + mapCol

			mapAddress := p.getMapAddress(mapType, mapIndex)
			if p.IsCGB {
				attributes := p.vram[1][mapAddress]
				palette = CGB_BGP0 + int(attributes&0x07)
				isXFlip = attributes&(1<<5) != 0
				isYFlip := attributes&(1<<6) != 0
				isCGBBGMapPriorityBitSet = attributes&(1<<7) != 0
				if isYFlip {
					data = p.getMapTile(mapAddress, 7-tilePixelY)
				} else {
					data = p.getMapTile(mapAddress, tilePixelY)
				}
			} else {
				palette = DMG_BGP
				data = p.getMapTile(mapAddress, tilePixelY)
			}
		}

		if mapType == Window {
			if mapPixelX < 0 || mapPixelX >= 160 {
				continue
			}
		}

		var lo, hi byte
		if isXFlip {
			lo = (data[0] >> tilePixelX) & 1
			hi = (data[1] >> tilePixelX) & 1
		} else {
			lo = (data[0] >> (7 - tilePixelX)) & 1
			hi = (data[1] >> (7 - tilePixelX)) & 1
		}
		colorIndex := (hi << 1) | lo
		p.pixelClues[pixelsIndexY+x].colorIndex = colorIndex
		p.pixelClues[pixelsIndexY+x].palette = palette
		p.pixelClues[pixelsIndexY+x].isCGBBGMapPriorityBitSet = isCGBBGMapPriorityBitSet

		if mapType == Window {
			isWindowDrawn = true
		}
	}
	return isWindowDrawn
}

// Get one row of tileData as background or window
func (p *PPU) getMapTile(mapAddress uint16, py int) [2]byte {
	bank := byte(0) // If DMG, always 0
	tileIndex := p.vram[0][mapAddress]

	// CGB mode only
	mapAttr := byte(0)
	if p.IsCGB {
		mapAttr = p.vram[1][mapAddress]
		bank = mapAttr & (1 << 3) >> 3
	}
	tileStart := uint16(0)
	if p.lcdc&(1<<4) != 0 { // Get tile data area
		tileStart = uint16(tileIndex) << 4
	} else {
		tileStart = uint16(0x1000 + int(int8(tileIndex))<<4)
	}
	tileAddr := tileStart + uint16(py*2)
	return [2]byte{
		p.vram[bank][tileAddr],
		p.vram[bank][tileAddr+1],
	}
}

func (p *PPU) getMapAddress(mapType, index int) uint16 {
	var tileMapArea int
	switch mapType {
	case Background:
		tileMapArea = int(p.lcdc & (1 << 3) >> 3)
	case Window:
		tileMapArea = int(p.lcdc & (1 << 6) >> 6)
	}
	var mapAddressStart uint16
	switch tileMapArea {
	case 0:
		mapAddressStart = 0x1800
	case 1:
		mapAddressStart = 0x1C00
	}
	mapAddress := mapAddressStart + uint16(index)
	return mapAddress
}

func (p *PPU) drawGameBoyScreen() {
	for y := 0; y < 144; y++ {
		for x := 0; x < 160; x++ {
			target := y*160 + x
			//
			if p.lcdc&(1<<7) == 0 {
				if p.IsCGB {
					p.screen.SetRGBA(x, y, color.RGBA{expand5bitLUT[31], expand5bitLUT[31], expand5bitLUT[31], 255})
				} else {
					p.screen.SetRGBA(x, y, p.dmgRGBAColorList[0])
				}
				continue
			}
			rgba := color.RGBA{255, 255, 255, 255}
			colorIndex := p.pixelClues[target].colorIndex // colorIndex = 0 ~ 3
			palette := p.pixelClues[target].palette

			// In this package, the palette index constants are assigned in the order DMG, CGB.
			if p.IsCGB { // If palette value is greater than DMG_OBP1, it is a CGB palette.
				var paletteOffset int
				var ram *[64]byte
				if palette >= CGB_BGP0 && palette < CGB_BGP0+8 {
					paletteOffset = palette - CGB_BGP0
					ram = &p.bgpRAM
				} else if palette >= CGB_OBP0 && palette < CGB_OBP0+8 {
					paletteOffset = palette - CGB_OBP0
					ram = &p.obpRAM
				}

				// One CGB palette size is 8 Bytes. One CGB color size is 2 Bytes.
				baseAddr := paletteOffset*8 + int(colorIndex)*2
				lo := uint16(ram[baseAddr])
				hi := uint16(ram[baseAddr+1])
				r := byte(lo & 0b00011111)
				g := byte(hi&0b00000011<<3 | lo&0b11100000>>5)
				b := byte(hi & 0b01111100 >> 2)
				// CGB pixels are converted from RGB555 format.
				rgba = color.RGBA{expand5bitLUT[r], expand5bitLUT[g], expand5bitLUT[b], 255}
			} else {
				var paletteRegister byte
				switch palette {
				case DMG_BGP:
					paletteRegister = p.bgp
				case DMG_OBP0:
					paletteRegister = p.obp0
				case DMG_OBP1:
					paletteRegister = p.obp1
				}
				finalGrayShadeIndex := paletteRegister >> (colorIndex * 2) & 0x03
				rgba = p.dmgRGBAColorList[finalGrayShadeIndex]

			}
			p.screen.SetRGBA(x, y, rgba)
		}
	}
}

// Get Viewport pixels converted from colorIndex to RGBA
func (p *PPU) GetGameScreen() *image.RGBA {
	return p.screen
}

func (p *PPU) ReadVRAM(addr uint16) byte {
	offset := addr & 0x1FFF // To prevent out of range errors
	return p.vram[p.vbk][offset]
}

func (p *PPU) WriteVRAM(addr uint16, val byte) {
	offset := addr & 0x1FFF // To prevent out of range errors
	p.vram[p.vbk][offset] = val
}

func (p *PPU) GetVBK() byte {
	return 0xFE | (p.vbk & 0x01)
}

func (p *PPU) SetVBK(val byte) {
	//fmt.Printf("VBK set to %d\n", val&0x01)
	p.vbk = val & 0x01
}

func (p *PPU) ReadOAM(addr uint16) byte {
	return p.oam[addr]
}
func (p *PPU) WriteOAM(addr uint16, val byte) {
	p.oam[addr] = val
}

func (p *PPU) GetDMA() byte {
	return p.dma
}

func (p *PPU) SetDMA(val byte) {
	p.dma = val
}

func (p *PPU) GetLCDC() byte {
	return p.lcdc
}

func (p *PPU) SetLCDC(val byte) {
	p.lcdc = val
}

func (p *PPU) GetSTAT() byte {
	return p.stat
}

func (p *PPU) SetSTAT(val byte) {
	p.stat = (val & 0x7C) | (p.stat & 0x03)

	// If any Mode int select is changed, check for interrupts
	p.checkSTATInt()
}

func (p *PPU) GetLY() byte {
	return p.ly
}

func (p *PPU) GetLYC() byte {
	return p.lyc
}

func (p *PPU) SetLYC(val byte) {
	p.lyc = val
}

func (p *PPU) GetOBP0() byte {
	return p.obp0
}

func (p *PPU) SetOBP0(val byte) {
	p.obp0 = val
}

func (p *PPU) GetOBP1() byte {
	return p.obp1
}

func (p *PPU) SetOBP1(val byte) {
	p.obp1 = val
}

func (p *PPU) GetBGP() byte {
	return p.bgp
}

func (p *PPU) SetBGP(val byte) {
	p.bgp = val
}

func (p *PPU) GetWY() byte {
	return p.wy
}

func (p *PPU) SetWY(val byte) {
	p.wy = val
}

func (p *PPU) GetWX() byte {
	return p.wx
}

func (p *PPU) SetWX(val byte) {
	p.wx = val
}

func (p *PPU) GetSCY() byte {
	return p.scy
}

func (p *PPU) SetSCY(val byte) {
	p.scy = val
}

func (p *PPU) GetSCX() byte {
	return p.scx
}

func (p *PPU) SetSCX(val byte) {
	p.scx = val
}

// (CGB mode only)
func (p *PPU) GetBCPS() byte {
	return p.bcps
}

// (CGB mode only)
func (p *PPU) SetBCPS(val byte) {
	p.bcps = val
}

// Read paletteRAM[BCPS.Address]
// (CGB mode only)
func (p *PPU) GetBCPD() byte {
	addr := p.bcps & 0x3F
	return p.bgpRAM[addr]
}

// Write to paletteRAM[BCPS.Address].
// And if BCPS.Auto-increment is enabled,
// increment BCPS.Address
// (CGB mode only)
func (p *PPU) SetBCPD(val byte) {
	addr := p.bcps & 0x3F
	p.bgpRAM[addr] = val
	if p.bcps&0x80 != 0 {
		newAddr := (addr + 1) & 0x3F
		p.bcps = 0x80 | newAddr
	}
}

// OCPS/OCPD exactly like BCPS/BCPD respectively
// (CGB mode only)
func (p *PPU) GetOCPS() byte {
	//fmt.Println("GetOCPS")
	return p.ocps
}
func (p *PPU) SetOCPS(val byte) {
	//fmt.Println("SetOCPS")
	p.ocps = val
}
func (p *PPU) GetOCPD() byte {
	//fmt.Println("GetOCPD")
	addr := p.ocps & 0x3F
	return p.obpRAM[addr]
}
func (p *PPU) SetOCPD(val byte) {
	//fmt.Println("SetOCPD")
	addr := p.ocps & 0x3F
	p.obpRAM[addr] = val
	if p.ocps&0x80 != 0 {
		newAddr := (addr + 1) & 0x3F
		p.ocps = 0x80 | newAddr
	}
}

// ====================================== HDMA Registers (CGB mode only) ==========================
// VRAM DMA Source (high)
func (p *PPU) SetHDMA1(val byte) {
	//fmt.Println("SetHDMA1")
	p.VDMASrc = uint16(val)<<8 | p.VDMASrc&0x00F0
}

// VRAM DMA Source (low)
func (p *PPU) SetHDMA2(val byte) {
	//fmt.Println("SetHDMA2")
	p.VDMASrc = p.VDMASrc&0xFF00 | uint16(val)&0x00F0
}

// VRAM DMA Destination (high)
func (p *PPU) SetHDMA3(val byte) {
	//fmt.Println("SetHDMA3")
	p.VDMADst = uint16(val)<<8&0x1F00 | p.VDMADst&0x00F0
}

// VRAM DMA Destination (low)
func (p *PPU) SetHDMA4(val byte) {
	//fmt.Println("SetHDMA4")
	p.VDMADst = p.VDMADst&0x1F00 | uint16(val)&0x00F0
}

// incomplete implementation
func (p *PPU) GetHDMA5() byte {
	//fmt.Println("GetHDMA5")
	if p.VDMALen == 0 {
		return 0xFF
	} else {
		return byte(p.VDMALen/0x10 - 1)
	}
}

// VRAM DMA length/mode/start
func (p *PPU) SetHDMA5(val byte) {
	//fmt.Println("SetHDMA5")
	p.VDMALen = (int(val)&0x7F + 1) * 0x10 // Therefore, transfer length == $10 ~ $800 Bytes
	//mode := p.hdma5 & 0x80 // 0 == General-purpose DMA        1 == HBlank DMA
	// Transfer is done via Bus
}

// CGB mode only
func (p *PPU) GetOPRI() byte {
	return 0xFE | p.opri&0x01
}

// CGB mode only
func (p *PPU) SetOPRI(val byte) {
	p.opri = 0xFE | val&0x01
}
