package ppu

import (
	"gomeboy/internal/util"
	"image/color"
	"math"
)

const (
	// DMG palette
	DMG_BGP  = 0
	DMG_OBP0 = 1
	DMG_OBP1 = 2

	// CGB color palette
	CGB_BGP0 = 3
	CGB_OBP0 = 11
)

var xFlipLUT [256]byte
var expand5bitLUT [32]byte

type PPU struct {
	// LCD
	vp [160 * 144]Pixel // Color Num

	// PPU Memory
	vram [2][0x2000]byte
	oam  [160]byte

	// I/O Registers (direct)
	bank byte
	dma  byte
	stat byte
	ly   byte
	lyc  byte
	obp0 byte
	obp1 byte
	bgp  byte
	wy   byte
	wx   byte
	scy  byte
	scx  byte
	bcps byte // CGB mode only
	bcpd byte // CGB mode only
	ocps byte // CGB mode only
	ocpd byte // CGB mode only

	// I/O Resisters (each split bit/option)
	// OPRI bit
	IsCGBstylePri bool // CGB mode only
	// LCDC bits
	isBGWEnabledOrPriority    bool // bit0 (different meaning in CGB mode)
	isOBJEnabled              bool // bit1
	isOBJBigSize              bool // bit2
	isSetBGTileMapAreaBit     bool // bit3
	isSetBGWTileDataAreaBit   bool // bit4
	isWindowEnabled           bool // bit5
	isSetWindowTileMapAreaBit bool // bit6
	isLCDEnabled              bool // bit7
	// BGP/OBP bits
	bgpData  [4]uint8 // Non-CGB mode only
	obp0Data [4]uint8 // Non-CGB mode only
	obp1Data [4]uint8 // Non-CGB mode only

	// Prev Registers
	isPrevWindowEnabled bool
	prevLY              byte

	// PPU Internal Counters
	wly    int
	cycles int

	// Interrupt
	HasSTATIRQ    bool
	HasVBlankIRQ  bool
	isLockLYCInt  bool
	isLockModeInt bool

	// Object
	objList []int

	bgpRAM       [64]byte // CGB mode only
	obpRAM       [64]byte // CGB mode only
	dmgColorRGBA [4]color.RGBA

	hasTransferRq bool
	IsCGB         bool

	// HDMA
	VDMASrc uint16 // CGB mode only
	VDMALen int    // CGB mode only
	VDMADst uint16 // CGB mode only
}

type Pixel struct {
	num      byte // 0 ~ 3
	pl       int  // BGP, PlaletteOBP0, OBP1
	isBGWPri bool //
}

func NewPPU() *PPU {
	createXFlipLUT()
	createExpand5bitLUT()
	p := &PPU{
		stat: 0x85,
		ly:   0x00,
		lyc:  0x00,
		wy:   0x00,
		wx:   0x00,
		scy:  0x00,
		scx:  0x00,
	}
	p.SetLCDC(0x91)
	p.SetBGP(0xFC)
	p.SetOBP0(0xFF)
	p.SetOBP1(0xFF)
	p.dmgColorRGBA = [4]color.RGBA{
		{255, 255, 128, 255},
		{160, 192, 64, 255},
		{64, 128, 64, 255},
		{0, 24, 0, 255},
	}
	return p
}

// For Object
func createXFlipLUT() {
	result := byte(0)
	for i := 0; i < 256; i++ {
		result = 0
		for b := 0; b < 8; b++ {
			if i&(1<<b) != 0 {
				result |= (1 << (7 - b))
			}
		}
		xFlipLUT[i] = result
	}
}

func createExpand5bitLUT() {
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

	// In case of PPU disabled
	if !p.isLCDEnabled {
		p.prevLY = p.ly
		p.ly = 0
		p.wly = 0
		return
	}

	// Check LYC==LY Interrupts
	p.stat = (p.stat &^ byte(1<<2)) // LYC==LY bit clear
	if p.lyc == p.ly {
		p.stat |= 1 << 2
		isLYCIntSel := (p.stat & byte(1<<6)) != 0
		if isLYCIntSel && !p.isLockLYCInt { // LYC==LY && LYCint enabled
			p.HasSTATIRQ = true
			p.isLockLYCInt = true
		}
	}

	switch {
	// During the period LY=0~143, repeat Mode2>3>0> every LY
	case p.ly <= 143:
		if p.ly != p.prevLY {
			p.hasTransferRq = true
		}
		// When starting with cycle=0, some settings may not be applied
		if p.hasTransferRq && p.cycles >= 280 {
			p.setPPUMode(2)
			p.oamSearch()
			p.setPPUMode(3)
			p.pixelTransfer()
			p.setPPUMode(0)
			p.hasTransferRq = false
		}
	// VBlank during LY=144~153 (VBlank IRQ occurs only once when LY=144)
	case p.ly == 144:
		if p.ly != p.prevLY {
			p.setPPUMode(1)
			p.HasVBlankIRQ = true
			p.wly = 0
		}
	}
	p.prevLY = p.ly

	p.checkSTATInt()

	// 1 Line == 456 CPU cycles
	p.cycles += cpuCycles
	if p.cycles >= 456 {
		p.cycles -= 456
		p.ly++
		p.isLockLYCInt = false
		if p.ly == 154 {
			p.ly = 0
		}
	}
}

// Update the frame buffer by one line
func (p *PPU) pixelTransfer() {
	// init vp
	/* base := int(p.ly) * 160
	for i := 0; i < 160; i++ {
		vp := &p.vp[base+i]
		vp.pl = 0
		vp.num = 0
	} */

	// BG/Window is On
	if p.isBGWEnabledOrPriority || p.IsCGB {

		// BG
		p.BGTransfer()

		// Window is On
		if p.isWindowEnabled {
			if !p.isPrevWindowEnabled {
				p.wly = 0
			}
			if p.ly >= p.wy {
				isDrawn := p.WindowTransfer()
				if isDrawn {
					p.wly++
				}
			}
		}
		p.isPrevWindowEnabled = p.isWindowEnabled
	}

	// Obj is On
	if p.isOBJEnabled {
		p.objectsTransfer()
	}
}

func (p *PPU) setPPUMode(nextMode int) {
	p.stat = (p.stat & 0x7C) | (byte(nextMode) & 0x03)
	p.isLockModeInt = false
}

func (p *PPU) checkSTATInt() {
	mode := p.stat & 0x03
	isIntEnabled := p.stat&(1<<(mode+3)) != 0
	if isIntEnabled && !p.isLockModeInt {
		p.HasSTATIRQ = true
		p.isLockModeInt = true
	}
}

// Lists 0~10 objects to be drawn on the current line
func (p *PPU) oamSearch() {
	p.objList = p.objList[:0] // =[]int{}

	bigMode := p.isOBJBigSize
	ly := int(p.ly)

	// List the objects in ascending OAM index order
	for i := 0; i < 40; i++ {

		// Y origin of Object(i) on the viewport
		y0 := int(p.oam[i<<2+0]) - 16

		var objHeight int
		if bigMode {
			objHeight = 16
		} else {
			objHeight = 8
		}

		// If part of the Y Position of Object(i) == LY, add it to the list
		if ly >= y0 && ly < (y0+objHeight) {
			p.objList = append(p.objList, i)
			// Ends when 10 objcets are found
			if len(p.objList) == 10 {
				return
			}
		}
	}
}

// Draw the current line objects listed by oamSearch() to vp
func (p *PPU) objectsTransfer() {
	ly := int(p.ly)

	// Sort the list X Position DESC or OAM index DESC
	sortedList := []int{}

	if p.IsCGB && p.IsCGBstylePri {
		for _, v := range p.objList {
			sortedList = util.InsertSlice(sortedList, 0, v)
		}
	} else {
		for _, oami := range p.objList {
			insPos := len(sortedList)
			for j, oamj := range sortedList {
				if p.oam[oami<<2+1] >= p.oam[oamj<<2+1] {
					insPos = j
					break
				}
			}
			sortedList = util.InsertSlice(sortedList, insPos, oami)
		}
	}

	// Draw objects to vp in the order sorted above
	for _, oamIdx := range sortedList {

		// Get Object attribytes in the OAM
		base := uint16(oamIdx << 2)
		y0 := int(p.oam[base]) - 16  // Byte 0 - Y Position
		x0 := int(p.oam[base+1]) - 8 // Byte 1 - X Position
		idx := p.oam[base+2]         // Byte 2 - Tile Index
		attr := p.oam[base+3]        // Byte 3 - Attributes/Flags
		tilePy := ly - y0

		data := [2]byte{}
		data = p.getObjectTile(idx, attr, tilePy)

		var pl int
		if p.IsCGB {
			pl = CGB_OBP0 + int(attr&0x07)
		} else {
			if (attr & (1 << 4)) == 0 {
				pl = DMG_OBP0
			} else {
				pl = DMG_OBP1
			}
		}

		// Draw tileData(one row) on vp
		tgtY := ly * 160
		for b := 0; b < 8; b++ {
			tgtX := x0 + b
			if tgtX < 0 || tgtX >= 160 {
				continue
			}
			tgt := tgtY + tgtX

			bgwNum := p.vp[tgt].num

			if p.IsCGB {
				if p.isBGWEnabledOrPriority && (bgwNum != 0 && p.vp[tgt].isBGWPri) {
					continue
				}
			}

			// In the case below, the Object pixel is hidden under the BG/W
			pri := (attr & (1 << 7)) >> 7
			if p.isBGWEnabledOrPriority && (pri == 1 && bgwNum != 0) {
				continue
			}
			// 2-bit monochrome (DMG)
			lo := (data[0] >> (7 - b)) & 1
			hi := (data[1] >> (7 - b)) & 1
			num := (hi << 1) | lo

			// When a tile is used in an object, ID 0 means transparent
			if num == 0 {
				continue
			}

			p.vp[tgt].pl = pl
			p.vp[tgt].num = num
		}
	}
}

// Get one row of tileData as an object
func (p *PPU) getObjectTile(idx, attr byte, tilePy int) [2]byte {
	isYFlip := attr&(1<<6) != 0
	isXFlip := attr&(1<<5) != 0
	bank := int(0)
	if p.IsCGB && attr&(1<<3) != 0 {
		bank = 1
	}
	if p.isOBJBigSize { // OBJ size
		idx &= 0xFE // When 8x16, idx is masked to even numbers only
		if isYFlip {
			tilePy = 15 - tilePy
		}
	} else {
		if isYFlip {
			tilePy = 7 - tilePy
		}
	}
	base := uint16(idx) << 4
	data := [2]byte{}
	for i := 0; i < 2; i++ {
		addr := base + uint16(tilePy<<1+i)
		data[i] = p.vram[bank][addr]
		if isXFlip {
			data[i] = xFlipLUT[data[i]]
		}
	}
	return data
}

// Draw the current line background to vp
func (p *PPU) BGTransfer() {
	var data [2]byte
	vpIdxY := int(p.ly) * 160
	bgY := int(p.ly + p.scy)
	bgRow := bgY >> 3    // =/8
	tilePy := bgY & 0x07 // =%8
	pl := DMG_BGP        // If DMG, Always BGP
	var isXFlip bool
	var isBGWPri bool

	for x := 0; x < 160; x++ {
		bgX := byte(x) + p.scx // wrapされる
		tilePx := bgX & 0x07   // =%8

		// Get tileData only when needed
		if x == 0 || tilePx == 0 {
			bgCol := int(bgX) >> 3
			bgIdx := bgRow<<5 + bgCol // <<5 == *32
			mapAddr := p.getBGWMapAddr(p.isSetBGTileMapAreaBit, bgIdx)
			if p.IsCGB {
				attr := p.vram[1][mapAddr]
				pl = CGB_BGP0 + int(attr&0x07)
				isXFlip = attr&(1<<5) != 0
				isYFlip := attr&(1<<6) != 0
				isBGWPri = attr&(1<<7) != 0
				if isYFlip {
					data = p.getBGWTile(mapAddr, 7-tilePy)
				} else {
					data = p.getBGWTile(mapAddr, tilePy)
				}
			} else {
				pl = DMG_BGP
				data = p.getBGWTile(mapAddr, tilePy)
			}
		}
		var lo, hi byte
		if isXFlip {
			lo = (data[0] >> tilePx) & 1
			hi = (data[1] >> tilePx) & 1
		} else {
			lo = (data[0] >> (7 - tilePx)) & 1
			hi = (data[1] >> (7 - tilePx)) & 1
		}
		num := (hi << 1) | lo
		p.vp[vpIdxY+x].num = num
		p.vp[vpIdxY+x].pl = pl
		p.vp[vpIdxY+x].isBGWPri = isBGWPri
	}
}

// Draw the current line Window to vp
func (p *PPU) WindowTransfer() bool {
	var isDrawn bool
	var data [2]byte
	vpIdxY := int(p.ly) * 160
	wRow := p.wly >> 3     // =/8
	tilePy := p.wly & 0x07 // =%8
	pl := DMG_BGP          // If DMG, Always BGP
	var isXFlip bool
	var isBGWPri bool

	for x := 0; x < 160; x++ {
		bgX := x - (int(p.wx) - 7)
		tilePx := bgX & 0x07 // =%8

		// Get tileData only when needed
		if x == 0 || tilePx == 0 {
			wCol := bgX >> 3
			wIdx := wRow<<5 + wCol // <<5 == *32
			mapAddr := p.getBGWMapAddr(p.isSetWindowTileMapAreaBit, wIdx)
			if p.IsCGB {
				attr := p.vram[1][mapAddr]
				pl = CGB_BGP0 + int(attr&0x07)
				isXFlip = attr&(1<<5) != 0
				isYFlip := attr&(1<<6) != 0
				isBGWPri = attr&(1<<7) != 0
				if isYFlip {
					data = p.getBGWTile(mapAddr, 7-tilePy)
				} else {
					data = p.getBGWTile(mapAddr, tilePy)
				}
			} else {
				pl = DMG_BGP
				data = p.getBGWTile(mapAddr, tilePy)
			}
		}
		if bgX < 0 || bgX >= 160 {
			continue
		}
		var lo, hi byte
		if isXFlip {
			lo = (data[0] >> tilePx) & 1
			hi = (data[1] >> tilePx) & 1
		} else {
			lo = (data[0] >> (7 - tilePx)) & 1
			hi = (data[1] >> (7 - tilePx)) & 1
		}
		num := (hi << 1) | lo
		p.vp[vpIdxY+x].num = num
		p.vp[vpIdxY+x].pl = pl
		p.vp[vpIdxY+x].isBGWPri = isBGWPri

		isDrawn = true
	}
	return isDrawn
}

// Get one row of tileData as background or window
func (p *PPU) getBGWTile(mapAddr uint16, py int) [2]byte {
	bank := byte(0) // If DMG, always 0
	tileIdx := p.vram[0][mapAddr]

	// CGB mode only
	mapAttr := byte(0)
	if p.IsCGB {
		mapAttr = p.vram[1][mapAddr]
		bank = (mapAttr & (1 << 3)) >> 3
	}
	tileStart := uint16(0)
	if p.isSetBGWTileDataAreaBit { // Get tile data area
		tileStart = uint16(tileIdx) << 4
	} else {
		tileStart = uint16(0x1000 + int(int8(tileIdx))<<4)
	}
	tileAddr := tileStart + uint16(py*2)
	return [2]byte{
		p.vram[bank][tileAddr],
		p.vram[bank][tileAddr+1],
	}
}

func (p *PPU) getBGWMapAddr(isSetTileMapAreaBit bool, idx int) uint16 {
	var mapStartAddr uint16
	if isSetTileMapAreaBit { // Get tile map area
		mapStartAddr = 0x1C00
	} else {
		mapStartAddr = 0x1800
	}
	mapAddr := mapStartAddr + uint16(idx)
	return mapAddr
}

// Get Viewport pixels converted from colorNum to RGBA
func (p *PPU) GetPixelsInRGBA(tgt int) color.RGBA {
	if !p.isLCDEnabled {
		if p.IsCGB {
			return color.RGBA{expand5bitLUT[31], expand5bitLUT[31], expand5bitLUT[31], 255}
		}
		return p.dmgColorRGBA[0]
	}
	rgba := color.RGBA{255, 255, 255, 255}
	num := p.vp[tgt].num // num = 0 ~ 3
	pl := p.vp[tgt].pl
	switch {
	case pl == DMG_BGP:
		color := p.bgpData[num]
		rgba = p.dmgColorRGBA[color]
	case pl == DMG_OBP0:
		color := p.obp0Data[num]
		rgba = p.dmgColorRGBA[color]
	case pl == DMG_OBP1:
		color := p.obp1Data[num]
		rgba = p.dmgColorRGBA[color]
	default: // = CGB
		onePlSize := 8
		oneColorSize := 2
		var baseAddr int
		var lo uint16
		var hi uint16
		if pl >= CGB_BGP0 && pl < CGB_BGP0+8 {
			baseAddr = (pl-CGB_BGP0)*onePlSize + int(num)*oneColorSize
			lo = uint16(p.bgpRAM[baseAddr])
			hi = uint16(p.bgpRAM[baseAddr+1])
		} else if pl >= CGB_OBP0 && pl < CGB_OBP0+8 {
			baseAddr = (pl-CGB_OBP0)*onePlSize + int(num)*oneColorSize
			lo = uint16(p.obpRAM[baseAddr])
			hi = uint16(p.obpRAM[baseAddr+1])
		}
		r := byte(lo & 0x1F)                                                         // lo0,1,2,3,4
		g := byte(((hi & 0x03) << 3) | ((lo & 0xE0) >> 5))                           // hi0,1 + lo5,6,7
		b := byte((hi & 0x7C) >> 2)                                                  // hi2,3,4,5,6
		rgba = color.RGBA{expand5bitLUT[r], expand5bitLUT[g], expand5bitLUT[b], 255} // RGB555 to RGBA
	}
	return rgba
}

/* // Also converts to RGBA
func (g *Game) updateImgFromFB(fb []byte) {
	srcBase := 0
	dstBase := 0
	for y := 0; y < 144; y++ {
		srcBase = y * 160
		dstBase = srcBase
		if isShowDebug {
			dstBase *= 2
		}
		for x := 0; x < 160; x++ {
			colorNum := fb[srcBase+x]
			dst := (dstBase + x) * 4
			var rgba []byte
			if g.emu.IsCGB {

			} else {
				rgba = g.palette[colorNum][:]
			}
			copy(g.img[dst:dst+4], rgba)
		}
	}
}
*/

func (p *PPU) ReadVRAM(addr uint16) byte {
	offset := addr & 0x1FFF // To prevent out of range errors
	return p.vram[p.bank][offset]
}

func (p *PPU) WriteVRAM(addr uint16, val byte) {
	offset := addr & 0x1FFF // To prevent out of range errors
	p.vram[p.bank][offset] = val
}

func (p *PPU) GetVBK() byte {
	return 0xFE | (p.bank & 0x01)
}

func (p *PPU) SetVBK(val byte) {
	//fmt.Printf("VBK set to %d\n", val&0x01)
	p.bank = val & 0x01
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
	v := byte(0)
	v |= util.BoolToByte(p.isBGWEnabledOrPriority) * 1     // bit0 (different meaning in CGB mode)
	v |= util.BoolToByte(p.isOBJEnabled) * 2               // bit1
	v |= util.BoolToByte(p.isOBJBigSize) * 4               // bit2
	v |= util.BoolToByte(p.isSetBGTileMapAreaBit) * 8      // bit3
	v |= util.BoolToByte(p.isSetBGWTileDataAreaBit) * 16   // bit4
	v |= util.BoolToByte(p.isWindowEnabled) * 32           // bit5
	v |= util.BoolToByte(p.isSetWindowTileMapAreaBit) * 64 // bit6
	v |= util.BoolToByte(p.isLCDEnabled) * 128             // bit7
	return v
}

func (p *PPU) SetLCDC(val byte) {
	p.isBGWEnabledOrPriority = val&(1<<0) != 0
	p.isOBJEnabled = val&(1<<1) != 0
	p.isOBJBigSize = val&(1<<2) != 0
	p.isSetBGTileMapAreaBit = val&(1<<3) != 0
	p.isSetBGWTileDataAreaBit = val&(1<<4) != 0
	p.isWindowEnabled = val&(1<<5) != 0
	p.isSetWindowTileMapAreaBit = val&(1<<6) != 0
	p.isLCDEnabled = val&(1<<7) != 0
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
	for i := 0; i < 4; i++ {
		p.obp0Data[i] = (val & (0x03 << (2 * uint8(i)))) >> (2 * uint8(i))
	}
	p.obp0 = val
}

func (p *PPU) GetOBP1() byte {
	return p.obp1
}

func (p *PPU) SetOBP1(val byte) {
	for i := 0; i < 4; i++ {
		p.obp1Data[i] = (val & (0x03 << (2 * uint8(i)))) >> (2 * uint8(i))
	}
	p.obp1 = val
}

func (p *PPU) GetBGP() byte {
	return p.bgp
}

func (p *PPU) SetBGP(val byte) {
	for i := 0; i < 4; i++ {
		p.bgpData[i] = (val & (0x03 << (2 * uint8(i)))) >> (2 * uint8(i))
	}
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
	p.VDMASrc = (uint16(val) << 8) | (p.VDMASrc & 0x00F0)
}

// VRAM DMA Source (low)
func (p *PPU) SetHDMA2(val byte) {
	//fmt.Println("SetHDMA2")
	p.VDMASrc = (p.VDMASrc & 0xFF00) | (uint16(val) & 0x00F0)
}

// VRAM DMA Destination (high)
func (p *PPU) SetHDMA3(val byte) {
	//fmt.Println("SetHDMA3")
	p.VDMADst = ((uint16(val) << 8) & 0x1F00) | (p.VDMADst & 0x00F0)
}

// VRAM DMA Destination (low)
func (p *PPU) SetHDMA4(val byte) {
	//fmt.Println("SetHDMA4")
	p.VDMADst = (p.VDMADst & 0x1F00) | (uint16(val) & 0x00F0)
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
	p.VDMALen = ((int(val) & 0x7F) + 1) * 0x10 // Therefore, transfer length == $10 ~ $800 Bytes
	//mode := p.hdma5 & 0x80 // 0 == General-purpose DMA        1 == HBlank DMA
	// Transfer is done via Bus
}

// CGB mode only
func (p *PPU) GetOPRI() byte {
	if p.IsCGBstylePri {
		return 0xFE
	} else {
		return 0xFF
	}
}

// CGB mode only
func (p *PPU) SetOPRI(val byte) {
	p.IsCGBstylePri = val&0x01 == 0
}
