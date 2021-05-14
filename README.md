# Renderers
# This repository contains:
# gomandel
A multithreaded Mandelbrot set renderer for creating zooming videos.

Adjust parameters on lines 179-189 inclusive. It renderes PNG images to an "out" folder in its directory.
I use the complex128 type, which allows for some fairly deep zooms but you will have to stop eventually.

## Usage

Just `go run .` or `go build .` Imports: `fmt, image, math, os, runtime, sync`

### Example output

https://www.youtube.com/watch?v=XI-YmIs3EpY

# And:
# oscilloscope
A multithreaded oscilloscope trace creator from a WAV file.
