// command inky runs on a raspberry pi and paints onto a pimoroni inky
// impression 7.3" eink display (ac073tc1a) plugged into its gpio headers.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/image/draw" // for draw.BiLinear
)

const (
	width  = 800
	height = 480
)

const (
	black = iota
	white
	green
	blue
	red
	yellow
	orange
)

// from the SATURATED_PALETTE in https://github.com/pimoroni/inky/blob/main/inky/inky_ac073tc1a.py
var palette = []color.Color{
	color.NRGBA{0, 0, 0, 255},       // black
	color.NRGBA{217, 242, 255, 255}, // white
	color.NRGBA{3, 124, 76, 255},    // green
	color.NRGBA{27, 46, 198, 255},   // blue
	color.NRGBA{245, 80, 34, 255},   // red
	color.NRGBA{255, 255, 68, 255},  // yellow
	color.NRGBA{239, 121, 44, 255},  // orange
}

// ac073tc1a instructions
const (
	PSR   = 0x00
	PWR   = 0x01
	POF   = 0x02
	POFS  = 0x03
	PON   = 0x04
	BTST1 = 0x05
	BTST2 = 0x06
	DSLP  = 0x07
	BTST3 = 0x08
	DTM   = 0x10
	DSP   = 0x11
	DRF   = 0x12
	IPC   = 0x13
	PLL   = 0x30
	TSC   = 0x40
	TSE   = 0x41
	TSW   = 0x42
	TSR   = 0x43
	CDI   = 0x50
	LPD   = 0x51
	TCON  = 0x60
	TRES  = 0x61
	DAM   = 0x65
	REV   = 0x70
	FLG   = 0x71
	AMV   = 0x80
	VV    = 0x81
	VDCS  = 0x82
	TVDCS = 0x84
	AGID  = 0x86
	CMDH  = 0xAA
	CCSET = 0xE0
	PWS   = 0xE3
	TSSET = 0xE6
)

func main() {
	log.SetFlags(0)
	outfile := flag.String("save", "", "save dithered image to file instead of eink screen")
	flag.Parse()

	path := flag.Arg(0)
	fi, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}
	if fi.IsDir() {
		de, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		images := []string{}
		for _, e := range de {
			if hasAnySuffix(e.Name(), ".jpg", ".jpeg", ".png", ".gif") {
				images = append(images, filepath.Join(path, e.Name()))
			}
		}
		path = images[rand.N(len(images))]
	}
	log.Printf("using %v", path)

	r, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	m, _, err := image.Decode(r)
	if err != nil {
		log.Fatal(err)
	}

	// scale & dither
	ratio := float64(m.Bounds().Dx()) / float64(m.Bounds().Dy())
	bounds := image.Rect((width-int(height*ratio))/2, 0, (width-int(height*ratio))/2+int(height*ratio), height)
	if ratio > width/height {
		bounds = image.Rect(0, (height-int(width/ratio))/2, width, (height-int(width/ratio))/2+int(width/ratio))
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(scaled, scaled.Bounds(), image.NewUniform(palette[white]), image.Point{}, draw.Src)
	draw.BiLinear.Scale(scaled, bounds, m, m.Bounds(), draw.Src, nil)
	dithered := image.NewPaletted(scaled.Bounds(), palette)
	draw.FloydSteinberg.Draw(dithered, dithered.Bounds(), scaled, image.Point{})

	if *outfile != "" {
		w, err := os.Create(*outfile)
		if err != nil {
			log.Fatal(err)
		}
		defer w.Close()
		err = png.Encode(w, dithered)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		spiConn, err := openSPI("/dev/spidev0.0", spi_mode_0, 5000000)
		if err != nil {
			log.Fatal(err)
		}
		// rpi pin 13 is broadcom gpio 27
		pinReset, err := openGPIO("/dev/gpiochip0", 27, gpio_v2_line_flag_output|gpio_v2_line_flag_bias_pull_down)
		if err != nil {
			log.Fatal(err)
		}
		// rpi pin 15 is broadcom gpio 22
		pinData, err := openGPIO("/dev/gpiochip0", 22, gpio_v2_line_flag_output|gpio_v2_line_flag_bias_pull_down)
		if err != nil {
			log.Fatal(err)
		}
		defer spiConn.Close()
		setup(spiConn, pinReset, pinData)
		paint(spiConn, pinData, shrink(dithered.Pix))
	}
}

func shrink(buf []byte) []byte {
	out := make([]byte, len(buf)/2)
	for i := range len(out) {
		out[i] = (buf[i*2] << 4) | buf[i*2+1]
	}
	return out
}

func demobuf() []byte {
	buf := make([]byte, width*height/2)
	for y := range height {
		for x := range width {
			i, _ := slices.BinarySearch([]int{0, 80, 160, 240, 320}, x-y)
			buf[(y*width+x)/2] |= []byte{white, red, green, blue, yellow, white}[i] << (4 * uint8(x&1))
		}
	}
	return buf
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func setup(spiConn *os.File, pinReset int32, pinData int32) {
	must(setGPIO(pinData, 0))

	// reset
	must(setGPIO(pinReset, 1))
	time.Sleep(100 * time.Millisecond)
	must(setGPIO(pinReset, 0))
	time.Sleep(100 * time.Millisecond)
	must(setGPIO(pinReset, 1))
	time.Sleep(1 * time.Second)

	// mystery init sequence from https://github.com/pimoroni/inky/blob/main/inky/inky_ac073tc1a.py
	log.Println("initialising screen")
	for _, step := range []struct {
		cmd  byte
		data []byte
	}{
		{CMDH, []byte{0x49, 0x55, 0x20, 0x08, 0x09, 0x18}},
		{PWR, []byte{0x3F, 0x00, 0x32, 0x2A, 0x0E, 0x2A}},
		{PSR, []byte{0x5F, 0x69}},
		{POFS, []byte{0x00, 0x54, 0x00, 0x44}},
		{BTST1, []byte{0x40, 0x1F, 0x1F, 0x2C}},
		{BTST2, []byte{0x6F, 0x1F, 0x16, 0x25}},
		{BTST3, []byte{0x6F, 0x1F, 0x1F, 0x22}},
		{IPC, []byte{0x00, 0x04}},
		{PLL, []byte{0x02}},
		{TSE, []byte{0x00}},
		{CDI, []byte{0x3F}},
		{TCON, []byte{0x02, 0x00}},
		{TRES, []byte{0x03, 0x20, 0x01, 0xE0}},
		{VDCS, []byte{0x1E}},
		{TVDCS, []byte{0x00}},
		{AGID, []byte{0x00}},
		{PWS, []byte{0x2F}},
		{CCSET, []byte{0x00}},
		{TSSET, []byte{0x00}},
	} {
		fmt.Fprintf(os.Stderr, ".")
		err := write(spiConn, pinData, step.cmd, step.data)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
}

func paint(spiConn *os.File, pinData int32, buf []byte) {
	log.Println("painting")
	for _, step := range []struct {
		cmd   byte
		data  []byte
		delay time.Duration
	}{
		{DTM, buf, time.Second},
		{PON, nil, time.Second},
		{DRF, []byte{0x00}, 35 * time.Second},
		{POF, []byte{0x00}, time.Second},
	} {
		fmt.Fprintf(os.Stderr, "..")
		err := write(spiConn, pinData, step.cmd, step.data)
		if err != nil {
			log.Fatal(err)
		}
		// there's a "busy" gpio pin to synchronise timing, but i found it unreliable.
		time.Sleep(step.delay)
	}
	fmt.Fprintf(os.Stderr, "\n")
}

func write(spiConn *os.File, pinData int32, cmd byte, data []byte) error {
	_, err := writeSPI(spiConn, []byte{cmd})
	if err != nil {
		return err
	}
	if data != nil {
		setGPIO(pinData, 1)
		defer setGPIO(pinData, 0)
		bufsiz := 4096
		for len(data) > 0 {
			chunk := min(len(data), bufsiz)
			_, err := writeSPI(spiConn, data[:chunk])
			if err != nil {
				return err
			}
			data = data[chunk:]
		}
	}
	return nil
}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}
