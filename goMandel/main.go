package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"runtime"
	"sync"
)

const rowsInJob = 400

var globalH, globalW int
var globalComputeJobs chan *computeJob

var globalWaitJobs sync.WaitGroup

type frameJob struct {
	frameID uint

	img *image.RGBA

	locsR []float64

	maxIter uint
	startR  float64
	lenR    float64

	startI float64
	lenI   float64

	baseLocI float64

	wg sync.WaitGroup
}

func newFrameJob(id, mi uint, sR, eR, sI, eI float64) *frameJob {
	return &frameJob{
		frameID: id,
		maxIter: mi,
		startR:  sR,
		lenR:    eR - sR,
		startI:  sI,
		lenI:    eI - sI,
	}
}

type computeJob struct {
	frame    *frameJob
	startRow int
	nRows    int
}

func (me *frameJob) newComputeJob(startY, nRows int) *computeJob {
	return &computeJob{
		frame:    me,
		startRow: startY,
		nRows:    nRows,
	}
}

func (me *computeJob) Run() {
	pixBuffer := me.frame.img.Pix

	for pixY := me.startRow + me.nRows - 1; pixY >= me.startRow; pixY-- {
		rOffs := pixY * me.frame.img.Stride
		rExt := rOffs + (globalW * 4)
		pixRow := pixBuffer[rOffs:rExt:rExt]

		valI := float64(pixY)*me.frame.lenI/float64(globalH) + me.frame.startI
		for pixX, valR := range me.frame.locsR {
			if res := mandel(complex(valR, valI), me.frame.maxIter); res < me.frame.maxIter {
				pixel := pixRow[pixX*4 : (pixX*4)+3 : (pixX*4)+3]
				Hp := (6.0 * float64(res)) / float64(me.frame.maxIter) // converts Hue to RGB
				X := uint8(math.Round((1.0 - math.Abs(math.Mod(Hp, 2.0)-1.0)) * 255.0))
				switch uint(Hp) {
				case 0:
					pixel[1] = X
					pixel[0] = 255
				case 1:
					pixel[1] = 255
					pixel[0] = X
				case 2:
					pixel[2] = X
					pixel[1] = 255
				case 3:
					pixel[2] = 255
					pixel[1] = X
				case 4:
					pixel[2] = 255
					pixel[0] = X
				default: // 5
					pixel[2] = X
					pixel[0] = 255
				}
			}
		}
	}
	me.frame.wg.Done()
}

func (me *frameJob) Run() {
	fmt.Printf("Running Frame %v at max_iter %v\n", me.frameID, me.maxIter)

	me.img = image.NewRGBA(image.Rect(0, 0, globalW, globalH))
	draw.Draw(me.img, me.img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.ZP, draw.Src)

	me.locsR = make([]float64, globalW) // a slice of Real values that need to be tested
	for co := range me.locsR {
		me.locsR[co] = float64(co)*me.lenR/float64(globalW) + me.startR
	}

	totaljobs := globalH / rowsInJob
	if globalH%rowsInJob > 0 {
		totaljobs++
	}

	me.wg.Add(totaljobs)
	for pixY := 0; pixY < globalH; pixY += rowsInJob {
		nRows := rowsInJob
		if pixY+rowsInJob > globalH {
			nRows = globalH - pixY
		}
		globalComputeJobs <- me.newComputeJob(pixY, nRows)
	}
	me.wg.Wait()
	me.saveImage()
}

func (me *frameJob) saveImage() {
	f, err := os.Create(fmt.Sprintf("out/%04v.png", me.frameID))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, me.img); err != nil {
		panic(err)
	}
	globalWaitJobs.Done()
}

func mandel(c complex128, maxI uint) uint {
	var z complex128
	for iter := uint(0); iter < maxI; iter++ {
		z = z*z + c

		r := real(z)
		i := imag(z)
		if math.Abs(r)+math.Abs(i) < float64(2) { // quick check to avoid expensive computation
			continue
		}
		if r*r+i*i >= float64(4) { // expensive computation but exact
			return maxI - iter
		}
	}
	return maxI
}

func computeWorker(jobs <-chan *computeJob) {
	for job := range jobs {
		job.Run()
	}
}

func frameWorker(jobs <-chan *frameJob) {
	for job := range jobs {
		job.Run()
	}
}

func main() {
	fmt.Println("Go Mandelbrot Renderer")

	// Parameters
	startFrame := uint(0)
	globalH = 1080                    // number of vertical lines
	ratio := float64(16) / float64(9) // aspect ratio
	startR := float64(-2.5)
	startI := float64(-1.15)
	maxIter := float64(100) // how many times to iterate the mandelbrot function
	totalFrames := uint(100)
	targetR := float64(-1.3852048812993896) // where to zoom to
	targetI := float64(0.012622046088551223)
	zoomSpeed := float64(0.2)
	iterStep := float64(5)

	// Everything else is calculated
	endI := math.Abs(startI)
	endR := startR + (ratio * endI * 2.0)
	globalW = int(math.Round(float64(globalH) * ratio))

	fmt.Printf("Resolution %vx%v r=%v\n", globalW, globalH, ratio)
	fmt.Printf("startR %v endR %v\n", startR, endR)
	fmt.Printf("startI %v endI %v\n", startI, endI)

	// Set up for multi-threading
	globalComputeJobs = make(chan *computeJob, 1000000)
	frameJobs := make(chan *frameJob, int(totalFrames))
	globalWaitJobs.Add(int(totalFrames - startFrame))

	for i := 0; i < runtime.NumCPU(); i++ {
		if i%2 == 0 {
			go frameWorker(frameJobs)
		}
		go computeWorker(globalComputeJobs)
	}

	for fNo := uint(0); fNo < totalFrames; fNo++ {
		if fNo >= startFrame {
			frameJobs <- newFrameJob(fNo, uint(maxIter), startR, endR, startI, endI)
		}

		startR += (targetR - startR) * zoomSpeed // adjust frame bounds to zoom
		endR += (targetR - endR) * zoomSpeed
		startI += (targetI - startI) * zoomSpeed
		endI += (targetI - endI) * zoomSpeed
		maxIter += iterStep
	}
	close(frameJobs)

	globalWaitJobs.Wait()

	close(globalComputeJobs)
}
