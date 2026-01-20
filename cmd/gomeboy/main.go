package main

import (
	"bytes"
	"fmt"
	"gomeboy/config"
	"gomeboy/internal/apu"
	"gomeboy/internal/emulator"
	"image"
	"image/color"
	"io"
	"log"
	"os"
	"strings"

	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

var isShowDebug bool
var cfg *config.Config

type Game struct {
	emu *emulator.Emulator
	img *ebiten.Image
	//img      []byte
	font     *text.GoTextFaceSource
	romTitle string
	scale    int
	audioCtx *audio.Context
	player   *audio.Player
	rc       io.ReadCloser
}

func NewGame(g *Game, rom, sav []byte) *Game {
	font, _ := text.NewGoTextFaceSource(bytes.NewReader(fonts.PressStart2P_ttf))

	debuggerWidth := 0
	if isShowDebug {
		debuggerWidth = 160
	}

	g.emu = emulator.NewEmulator(rom, sav)
	g.img = ebiten.NewImage(160+debuggerWidth, 144)
	//g.img = make([]byte, imgSize)
	g.font = font

	// Pass Joypad-related settings in config.toml
	g.emu.CPU.Bus.Joypad.SetIsGamepadEnabled(cfg.Gamepad.IsEnabled)
	g.emu.CPU.Bus.Joypad.SetIsGamepadBind(cfg.Gamepad.Bind)

	g.audioCtx = audio.NewContext(apu.SampleRate)
	g.player, _ = g.audioCtx.NewPlayer(g.emu.CPU.Bus.APU.AudioStream)
	//g.player.SetBufferSize(1024 * 4)
	g.player.SetVolume(0.2)
	g.player.Play()

	return g
}

func (g *Game) Update() error {
	if g.player == nil {
	}
	g.setWindowTitle()
	if g.emu.RunFrame() == -1 {
		return ebiten.Termination
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {

	g.img = ebiten.NewImageFromImage(g.getViewportRGBAFormatted())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(g.scale), float64(g.scale))
	/* shaderOp := &ebiten.DrawRectShaderOptions{}
	shaderOp.Uniforms = map[string]interface{}{
		"PixelSize": float32(160*2),
	}
	dotShader := ebiten.NewShader(g.getViewportRGBAFormatted())
	screen.DrawRectShader(160*3*2, 144, dotShader, shaderOp) */
	screen.DrawImage(g.img, op)
	//screen.WritePixels(g.img)
	if isShowDebug {
		g.drawDebugMonitor(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Internal Resolution (Stretch to fit window size while maintaining aspect ratio)
	screenHeight := 144 * g.scale
	screenWidth := 160 * g.scale
	if isShowDebug {
		screenWidth *= 2
	}
	return screenWidth, screenHeight
}

func main() {
	// Load config.toml settings
	var err error
	if cfg, err = config.Load("config.toml"); err != nil {
		panic(err)
	}
	g := &Game{}

	g.scale = cfg.Video.Scale
	g.scale = max(g.scale, 0)
	g.scale = min(g.scale, 4)
	isShowDebug = cfg.Video.IsShowDebug

	// Load .gb file
	if len(os.Args) < 2 {
		fmt.Println("usage: gomeboy <romfile>")
		return
	}
	romPath := os.Args[1]
	rom, err := os.ReadFile(romPath)
	if err != nil {
		log.Fatal(err)
	}

	// Load .sav file (if exists)
	savPath := getSavePathFromROM(romPath)
	sav, _ := os.ReadFile(savPath)

	// Set window size
	windowHeight := 144 * g.scale
	windowWidth := 160 * g.scale
	if isShowDebug {
		windowWidth *= 2
	}
	ebiten.SetWindowSize(windowWidth, windowHeight)

	// Get ROM title
	s := string(rom[0x0134:0x0144])
	firstNullIdx := strings.IndexByte(s, 0)
	if firstNullIdx != -1 {
		s = s[:firstNullIdx]
	}
	g.romTitle = s

	if err := ebiten.RunGame(NewGame(g, rom, sav)); err != nil && err != ebiten.Termination {
		panic(err)
	} else {
		// When the emulator is closed, save ERAM(save) data
		savData := g.emu.CPU.Bus.Memory.GetSaveData()
		os.WriteFile(savPath, savData, 0644)
	}
}

// Output to the right of the screen.
func (g *Game) drawDebugMonitor(screen *ebiten.Image) {
	strs := []string{} // Max 20chars * 18rows
	state := string("     ")
	if g.emu.IsPaused {
		state = "PAUSE"
	}
	top := fmt.Sprintf(state+"        FPS:%3.0f", ebiten.ActualFPS())
	strs = append(strs, top)
	strs = append(strs, g.emu.CPU.Tracer.GetCPUInfo()...)
	strs = append(strs, "")
	strs = append(strs, g.emu.CPU.Bus.Memory.GetHeaderInfo()...)
	strs = append(strs, "")
	strs = append(strs, g.emu.CPU.Bus.APU.GetAPUInfo()...)
	white := color.RGBA{255, 255, 255, 255}
	red := color.RGBA{255, 0, 0, 255}
	var cr = color.RGBA{}
	for i, s := range strs {
		if i == 0 {
			cr = red
		} else {
			cr = white
		}
		g.drawText(screen, s, 160*g.scale, i*16, 16, cr)
	}
}

func (g *Game) getViewportRGBAFormatted() *image.RGBA {
	idxScale := 1
	if isShowDebug {
		idxScale = 2
	}
	img := image.NewRGBA(image.Rect(0, 0, 160*idxScale, 144))

	base := 0
	rgba := color.RGBA{}
	for y := 0; y < 144; y++ {
		base = y * 160
		for x := 0; x < 160; x++ {
			rgba = g.emu.CPU.Bus.PPU.GetPixelsInRGBA(base + x)
			/* dst := (base*idxScale + x) * 4
			copy(g.img[dst:dst+4], rgba[:]) */
			img.SetRGBA(x, y, rgba)
		}
	}
	return img
}

// Use instead of ebiten.DebugPrint
func (g *Game) drawText(dst *ebiten.Image, msg string, x, y, size int, cr color.RGBA) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(cr)
	text.Draw(dst, msg, &text.GoTextFace{
		Source: g.font,
		Size:   float64(size),
	}, op)
}

func getSavePathFromROM(romPath string) string {
	ext := filepath.Ext(romPath)
	base := romPath[:len(romPath)-len(ext)]
	return base + ".sav"
}

func (g *Game) setWindowTitle() {
	emuState := ""
	if g.emu.IsPaused {
		emuState = "(paused)"
	}
	if len(g.romTitle) > 0 {
		ebiten.SetWindowTitle(emuState + "GOmeBoy - " + g.romTitle)
	} else {
		ebiten.SetWindowTitle(emuState + "GOmeBoy")
	}
}
