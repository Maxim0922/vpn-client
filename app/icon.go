package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func trayIcon(connected bool) []byte {
	const s = 44
	img := image.NewNRGBA(image.Rect(0, 0, s, s))
	black := color.NRGBA{0, 0, 0, 255}
	clear := color.NRGBA{0, 0, 0, 0}

	cx := float64(s) / 2
	for y := 0; y < s; y++ {
		for x := 0; x < s; x++ {
			if inShield(float64(x), float64(y), cx, s) {
				img.SetNRGBA(x, y, black)
			} else {
				img.SetNRGBA(x, y, clear)
			}
		}
	}
	if connected {
		punchCheck(img, s)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func inShield(x, y, cx float64, s float64) bool {
	top := s * 0.12
	bottom := s * 0.90
	halfW := s * 0.34
	if y < top || y > bottom {
		return false
	}

	t := (y - top) / (bottom - top)
	w := halfW * (1.0 - 0.85*t*t)
	dx := x - cx
	return dx >= -w && dx <= w
}

func punchCheck(img *image.NRGBA, s int) {
	clear := color.NRGBA{0, 0, 0, 0}
	fs := float64(s)

	seg := func(x0, y0, x1, y1, th float64) {
		steps := 60
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps)
			px := x0 + (x1-x0)*t
			py := y0 + (y1-y0)*t
			for oy := -th; oy <= th; oy++ {
				for ox := -th; ox <= th; ox++ {
					xi, yi := int(px+ox), int(py+oy)
					if xi >= 0 && yi >= 0 && xi < s && yi < s {
						img.SetNRGBA(xi, yi, clear)
					}
				}
			}
		}
	}
	seg(fs*0.34, fs*0.50, fs*0.46, fs*0.62, fs*0.035)
	seg(fs*0.46, fs*0.62, fs*0.68, fs*0.36, fs*0.035)
}
