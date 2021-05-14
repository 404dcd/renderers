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

const FPS = 30

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

func realisticDraw(img *image.RGBA, x, y int, br uint8) {
	start := (y * img.Stride) + (x * 4)
	if start > 4194303 {
		return
	}
	curbr := img.Pix[start]
	if curbr == 0 {
		img.Pix[start+3] = 255
	}
	if 255-curbr <= br {
		img.Pix[start] = 255
		img.Pix[start+1] = 255
		img.Pix[start+2] = 255
	} else {
		br += curbr
		img.Pix[start] = br
		img.Pix[start+1] = br
		img.Pix[start+2] = br
	}
}

func Bresenham(img *image.RGBA, x1, y1, x2, y2 int, br uint8) {
	var dx, dy, e, slope int

	if x1 > x2 {
		x1, y1, x2, y2 = x2, y2, x1, y1
	}

	dx, dy = x2-x1, y2-y1
	if dy < 0 {
		dy = -dy
	}

	switch {

	case x1 == x2 && y1 == y2:
		realisticDraw(img, x1, y1, br)

	case y1 == y2:
		for ; dx != 0; dx-- {
			realisticDraw(img, x1, y1, br)
			x1++
		}
		realisticDraw(img, x1, y1, br)

	case x1 == x2:
		if y1 > y2 {
			y1 = y2
		}
		for ; dy != 0; dy-- {
			realisticDraw(img, x1, y1, br)
			y1++
		}
		realisticDraw(img, x1, y1, br)

	case dx == dy:
		if y1 < y2 {
			for ; dx != 0; dx-- {
				realisticDraw(img, x1, y1, br)
				x1++
				y1++
			}
		} else {
			for ; dx != 0; dx-- {
				realisticDraw(img, x1, y1, br)
				x1++
				y1--
			}
		}
		realisticDraw(img, x1, y1, br)

	case dx > dy:
		if y1 < y2 {
			dy, e, slope = 2*dy, dx, 2*dx
			for ; dx != 0; dx-- {
				realisticDraw(img, x1, y1, br)
				x1++
				e -= dy
				if e < 0 {
					y1++
					e += slope
				}
			}
		} else {
			dy, e, slope = 2*dy, dx, 2*dx
			for ; dx != 0; dx-- {
				realisticDraw(img, x1, y1, br)
				x1++
				e -= dy
				if e < 0 {
					y1--
					e += slope
				}
			}
		}
		realisticDraw(img, x2, y2, br)

	default:
		if y1 < y2 {
			dx, e, slope = 2*dx, dy, 2*dy
			for ; dy != 0; dy-- {
				realisticDraw(img, x1, y1, br)
				y1++
				e -= dx
				if e < 0 {
					x1++
					e += slope
				}
			}
		} else {
			dx, e, slope = 2*dx, dy, 2*dy
			for ; dy != 0; dy-- {
				realisticDraw(img, x1, y1, br)
				y1--
				e -= dx
				if e < 0 {
					x1++
					e += slope
				}
			}
		}
		realisticDraw(img, x2, y2, br)
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

		Bresenham(img, lastx, lasty, x, y, uint8(bri))
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
