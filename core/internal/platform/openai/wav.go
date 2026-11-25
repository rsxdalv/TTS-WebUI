package openai

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

// wavInfo is what Go needs of a WAV file: to tell it is one, to hash and time
// the samples and to draw the waveform that History shows as the picture.
type wavInfo struct {
	format   uint16 // 1 integer PCM, 3 IEEE float
	channels int
	rate     int
	bits     int
	data     []byte // the sample bytes
}

var errNotWav = errors.New("not a WAV file")

// parseWav reads a RIFF/WAVE file. A data chunk whose size is 0 or 0xFFFFFFFF
// (a streamed WAV) runs to the end of the file.
func parseWav(b []byte) (wavInfo, error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return wavInfo{}, errNotWav
	}
	var w wavInfo
	haveFmt := false
	for p := 12; p+8 <= len(b); {
		id := string(b[p : p+4])
		size := int(binary.LittleEndian.Uint32(b[p+4 : p+8]))
		body := p + 8
		switch id {
		case "fmt ":
			if size < 16 || body+16 > len(b) {
				return wavInfo{}, errors.New("WAV fmt chunk is too short")
			}
			w.format = binary.LittleEndian.Uint16(b[body:])
			w.channels = int(binary.LittleEndian.Uint16(b[body+2:]))
			w.rate = int(binary.LittleEndian.Uint32(b[body+4:]))
			w.bits = int(binary.LittleEndian.Uint16(b[body+14:]))
			if w.format == 0xFFFE && size >= 26 && body+26 <= len(b) {
				w.format = binary.LittleEndian.Uint16(b[body+24:]) // the sub-format
			}
			haveFmt = true
		case "data":
			if !haveFmt {
				return wavInfo{}, errors.New("WAV data chunk before the fmt chunk")
			}
			end := body + size
			if size == 0 || size == 0xFFFFFFFF || end > len(b) {
				end = len(b)
			}
			w.data = b[body:end]
			if w.channels < 1 || w.rate < 1 {
				return wavInfo{}, errors.New("WAV header has no channels or sample rate")
			}
			if !w.supported() {
				return wavInfo{}, fmt.Errorf("WAV sample format %d with %d bits is not supported", w.format, w.bits)
			}
			return w, nil
		}
		p = body + size + size%2
	}
	return wavInfo{}, errors.New("WAV has no data chunk")
}

func (w wavInfo) supported() bool {
	switch w.format {
	case 1:
		return w.bits == 8 || w.bits == 16 || w.bits == 24 || w.bits == 32
	case 3:
		return w.bits == 32
	}
	return false
}

func (w wavInfo) frameSize() int { return w.channels * w.bits / 8 }

// Frames is the number of sample frames.
func (w wavInfo) frames() int { return len(w.data) / w.frameSize() }

// seconds is the clip length.
func (w wavInfo) seconds() float64 { return float64(w.frames()) / float64(w.rate) }

// sample returns the amplitude (-1..1) of channel c of frame i.
func (w wavInfo) sample(i, c int) float64 {
	bytesPer := w.bits / 8
	off := i*w.frameSize() + c*bytesPer
	d := w.data[off : off+bytesPer]
	switch {
	case w.format == 3:
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(d)))
	case w.bits == 8:
		return (float64(d[0]) - 128) / 128
	case w.bits == 16:
		return float64(int16(binary.LittleEndian.Uint16(d))) / 32768
	case w.bits == 24:
		v := int32(d[0]) | int32(d[1])<<8 | int32(int8(d[2]))<<16
		return float64(v) / 8388608
	default:
		return float64(int32(binary.LittleEndian.Uint32(d))) / 2147483648
	}
}

// peaks returns the largest amplitude (0..1) of each of n equal slices of the clip.
func (w wavInfo) peaks(n int) []float64 {
	out := make([]float64, n)
	frames := w.frames()
	if frames == 0 {
		return out
	}
	for col := 0; col < n; col++ {
		from, to := col*frames/n, (col+1)*frames/n
		if to <= from {
			to = from + 1
		}
		if to > frames {
			to = frames
		}
		peak := 0.0
		for i := from; i < to; i++ {
			for c := 0; c < w.channels; c++ {
				if a := math.Abs(w.sample(i, c)); a > peak {
					peak = a
				}
			}
		}
		out[col] = math.Min(peak, 1)
	}
	return out
}

// waveformPNG draws the clip as the 1000 x 300 picture the History page shows
// next to the audio (the legacy generators save a plot of the same size).
func (w wavInfo) waveformPNG() ([]byte, error) {
	const width, height = 1000, 300
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bg := color.RGBA{255, 255, 255, 255}
	fg := color.RGBA{31, 119, 180, 255}
	mid := color.RGBA{200, 200, 200, 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	for x := 0; x < width; x++ {
		img.SetRGBA(x, height/2, mid)
	}
	for x, p := range w.peaks(width) {
		half := int(p * float64(height/2-4))
		for y := height/2 - half; y <= height/2+half; y++ {
			img.SetRGBA(x, y, fg)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
