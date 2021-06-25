package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"runtime"
	"sync"
	"time"
)

const FPS = 60

type WAVfile struct {
	filepath   string
	channels   uint16
	samplerate uint32
	bps        uint16
	samples    uint32
	reader     *bufio.Reader
}

type lrLevel struct {
	left  int16
	right int16
}

func (me *WAVfile) ReadSample() (*lrLevel, error) {
	var ret lrLevel
	buf := make([]byte, me.bps/8)
	n, err := me.reader.Read(buf)
	if err != nil {
		return nil, err
	}
	if uint16(n) != me.bps/8 {
		return nil, errors.New("malformed WAV file")
	}

	if me.bps == 8 {
		ret.left = int16(buf[0])
	} else if me.bps == 16 {
		ret.left = int16(binary.LittleEndian.Uint16(buf))
	} else {
		return nil, errors.New("WAV data not supported")
	}

	if me.channels == 1 {
		ret.right = ret.left
		return &ret, nil
	} else if me.channels == 2 {
		n, err := me.reader.Read(buf)
		if err != nil {
			return nil, err
		}
		if uint16(n) != me.bps/8 {
			return nil, errors.New("malformed WAV file")
		}
		if me.bps == 8 {
			ret.right = int16(buf[0])
		} else if me.bps == 16 {
			ret.right = int16(binary.LittleEndian.Uint16(buf))
		} else {
			return nil, errors.New("WAV data not supported")
		}
		return &ret, nil
	} else {
		return nil, errors.New("WAV data not supported")
	}
}

func NewWAVfile(fp string) (*WAVfile, error) {
	var me WAVfile
	var n int
	buf2 := make([]byte, 2)
	buf4 := make([]byte, 4)
	readCheck := func(number int) (err error) {
		if number == 2 {
			n, err = me.reader.Read(buf2)
			if err != nil {
				return err
			}
			if n != 2 {
				return errors.New("malformed WAV file")
			}
		} else {
			n, err = me.reader.Read(buf4)
			if err != nil {
				return err
			}
			if n != 4 {
				return errors.New("malformed WAV file")
			}
		}
		return nil
	}
	me.filepath = fp
	f, err := os.Open("test.wav")
	if err != nil {
		return &me, err
	}
	me.reader = bufio.NewReader(f)

	if err := readCheck(4); err != nil {
		return &me, err
	}
	if string(buf4) != "RIFF" {
		return &me, errors.New("malformed WAV file")
	}

	if err := readCheck(4); err != nil {
		return &me, err
	}

	if err := readCheck(4); err != nil {
		return &me, err
	}
	if string(buf4) != "WAVE" {
		return &me, errors.New("malformed WAV file")
	}

	if err := readCheck(4); err != nil {
		return &me, err
	}
	if string(buf4) != "fmt " {
		return &me, errors.New("malformed WAV file")
	}

	if err := readCheck(4); err != nil {
		return &me, err
	}
	if binary.LittleEndian.Uint32(buf4) != 16 {
		return &me, errors.New("malformed WAV file")
	}

	if err := readCheck(2); err != nil {
		return &me, err
	}
	if binary.LittleEndian.Uint16(buf2) != 1 {
		return &me, errors.New("this WAV format is not supported")
	}

	if err := readCheck(2); err != nil {
		return &me, err
	}
	me.channels = binary.LittleEndian.Uint16(buf2)

	if err := readCheck(4); err != nil {
		return &me, err
	}
	me.samplerate = binary.LittleEndian.Uint32(buf4)

	if err := readCheck(4); err != nil {
		return &me, err
	}

	if err := readCheck(2); err != nil {
		return &me, err
	}

	if err := readCheck(2); err != nil {
		return &me, err
	}
	me.bps = binary.LittleEndian.Uint16(buf2)

	if err := readCheck(4); err != nil {
		return &me, err
	}
	if string(buf4) != "data" {
		return &me, errors.New("malformed WAV file")
	}

	if err := readCheck(4); err != nil {
		return &me, err
	}
	me.samples = binary.LittleEndian.Uint32(buf4) / uint32(me.channels) / uint32(me.bps) * 8

	return &me, nil
}

func lineDraw(img *image.RGBA, x1, y1, x2, y2 int, br uint8) {
	dy := y2 - y1
	dx := x2 - x1
	f64dx := float64(dx)
	tmp := f64dx / float64(dy<<1)
	if tmp < 0 {
		tmp *= -1
	}

	subtr := 0.5 - tmp
	if subtr < 0 {
		subtr *= -1
	}
	stridex := 0.5 + tmp - subtr
	stridey := stridex * float64(dy) / f64dx

	tmp2 := dx >> 63
	dx = (dx ^ tmp2) - tmp2
	tmp2 = dy >> 63
	dy = (dy ^ tmp2) - tmp2

	x := float64(x1+x2-dx) / 2
	y := float64(y1+y2-dy) / 2
	if stridey < 0 {
		y = float64(y1+y2+dy) / 2
	}
	boundx := (x1 + x2 + dx) / 2
	boundy := (y1 + y2 + dy) / 2
	intx := int(x)
	inty := int(y)
	if dx == 0 {
		stridex = 0
		stridey = 1
	}
	if dy == 0 {
		stridex = 1
		stridey = 0
	}
	for intx <= boundx && inty <= boundy {
		start := (inty * img.Stride) + (intx * 4)
		brcp := br
		if start > 4194303 {
			continue
		}
		curbr := img.Pix[start]
		if curbr == 0 {
			img.Pix[start+3] = 255
		}
		if 255-curbr <= brcp {
			img.Pix[start] = 255
			img.Pix[start+1] = 255
			img.Pix[start+2] = 255
		} else {
			brcp += curbr
			img.Pix[start] = brcp
			img.Pix[start+1] = brcp
			img.Pix[start+2] = brcp
		}
		y += stridey
		x += stridex
		intx = int(x + 0.5)
		inty = int(y + 0.5)
	}
}

func plotter(pxs chan *lrLevel, frameid uint32, wg *sync.WaitGroup) {
	defer wg.Done()
	img := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	var x, y, lastx, lasty int
	px := <-pxs
	x = int(px.left>>6) + 512
	y = int(px.right>>6) + 512
	for px = range pxs {
		lastx = x
		lasty = y
		x = int(px.left>>6) + 512
		y = 512 - int(px.right>>6)

		distance := float64((lastx - x) * (lastx - x))
		distance += float64((lasty - y) * (lasty - y))
		bri := 255 - math.Pow(distance, 0.6)
		if bri < 0 {
			bri = 0
		}

		_ = lineDraw //(img, lastx, lasty, x, y, uint8(bri))
	}
	f, err := os.Create(fmt.Sprintf("out/img%04v.png", frameid))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func main() {
	wf, err := NewWAVfile("test.wav")
	if err != nil {
		panic(err)
	}
	samplesPerFrame := wf.samplerate / FPS
	var wg sync.WaitGroup

	process := make(chan *lrLevel, samplesPerFrame)
	wg.Add(1)
	go plotter(process, 0, &wg)

	for sample := uint32(0); sample < wf.samples; sample++ {
		levels, err := wf.ReadSample()
		if err != nil {
			panic(err)
		}

		process <- levels

		if sample%samplesPerFrame == 0 {
			fmt.Printf("%f%%\r", float32(sample*100)/float32(wf.samples))
			close(process)
			process = make(chan *lrLevel, samplesPerFrame)
			for runtime.NumGoroutine() > 32 {
				time.Sleep(10 * time.Millisecond)
			}
			wg.Add(1)
			go plotter(process, sample/samplesPerFrame, &wg)
		}
	}
	close(process)
	wg.Wait()
}
