package widgets

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/d093w1z/gio/f32"
	"github.com/d093w1z/gio/layout"
	"github.com/d093w1z/gio/op"
	"github.com/d093w1z/gio/op/clip"
	"github.com/d093w1z/gio/op/paint"
	"github.com/d093w1z/gio/text"
	"github.com/d093w1z/gio/unit"
	"github.com/d093w1z/gio/widget"
	"github.com/d093w1z/gio/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func formatDuration(d time.Duration) string {
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func ProgressArc(gtx layout.Context, remaining, total time.Duration) layout.Dimensions {
	size := gtx.Dp(unit.Dp(200))
	center := f32.Point{X: float32(size) / 2, Y: float32(size) / 2}
	radius := float32(size/2 - gtx.Dp(unit.Dp(10)))

	progress := float32(remaining.Seconds() / total.Seconds())
	angle := 2 * math.Pi * progress
	// startAngle := -math.Pi / 2 // starting from top

	f1 := f32.Point{X: center.X - radius, Y: center.Y}
	f2 := f32.Point{X: center.X + radius, Y: center.Y}

	var ops op.Ops = *gtx.Ops
	var path clip.Path
	path.Begin(&ops)
	path.MoveTo(center)
	path.Arc(f1, f2, angle) // builds arc segment
	spec := path.End()

	// Gradient paint (linear for example)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(center.X-radius, center.Y),
		Stop2:  f32.Pt(center.X+radius, center.Y),
		Color1: color.NRGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF},
		Color2: color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF},
	}.Add(gtx.Ops)

	paint.FillShape(gtx.Ops, color.NRGBA{A: 0xFF}, clip.Outline{Path: spec}.Op())

	return layout.Dimensions{Size: image.Pt(size, size)}
}

// Linear interpolation of colors
func lerpColor(c1, c2 color.NRGBA, t float32) color.NRGBA {
	return color.NRGBA{
		R: uint8(float32(c1.R) + t*(float32(c2.R)-float32(c1.R))),
		G: uint8(float32(c1.G) + t*(float32(c2.G)-float32(c1.G))),
		B: uint8(float32(c1.B) + t*(float32(c2.B)-float32(c1.B))),
		A: uint8(float32(c1.A) + t*(float32(c2.A)-float32(c1.A))),
	}
}
func DrawGradientRing(gtx layout.Context, progress float32, startColor, endColor color.NRGBA) layout.Dimensions {
	size := gtx.Dp(unit.Dp(200))
	center := float32(size) / 2
	outerRadius := center
	innerRadius := outerRadius - 10 // thickness

	// Use fewer segments for smoother arcs
	segments := 60
	maxSeg := int(float32(segments) * progress)

	if maxSeg == 0 {
		return layout.Dimensions{Size: image.Pt(size, size)}
	}

	angleStep := 2 * math.Pi / float64(segments)
	segmentAngle := float32(angleStep)

	for i := 0; i < maxSeg; i++ {
		startAngle := float32(i)*segmentAngle - math.Pi/2 // Start from top
		endAngle := startAngle + segmentAngle

		// Calculate gradient color for this segment
		// Key change: interpolate only within the drawn arc (0 to maxSeg)
		t := float32(i) / float32(maxSeg-1)
		if maxSeg == 1 {
			t = 0
		}
		c := lerpColor(startColor, endColor, t)

		// Create path for the ring segment
		var p clip.Path
		p.Begin(gtx.Ops)

		// Calculate points
		startCos, startSin := math.Cos(float64(startAngle)), math.Sin(float64(startAngle))
		endCos, endSin := math.Cos(float64(endAngle)), math.Sin(float64(endAngle))

		// Outer arc start point
		outerStartX := center + outerRadius*float32(startCos)
		outerStartY := center + outerRadius*float32(startSin)

		// Outer arc end point
		outerEndX := center + outerRadius*float32(endCos)
		outerEndY := center + outerRadius*float32(endSin)

		// Inner arc start point
		innerStartX := center + innerRadius*float32(startCos)
		innerStartY := center + innerRadius*float32(startSin)

		// Inner arc end point
		innerEndX := center + innerRadius*float32(endCos)
		innerEndY := center + innerRadius*float32(endSin)

		// Build the path
		p.MoveTo(f32.Pt(outerStartX, outerStartY))

		// Outer arc - use QuadTo for smoother curves
		if segmentAngle <= math.Pi {
			// For segments <= 180°, use a single arc
			midAngle := startAngle + segmentAngle/2
			midCos, midSin := math.Cos(float64(midAngle)), math.Sin(float64(midAngle))

			// Control point for quadratic bezier to approximate arc
			controlRadius := outerRadius / float32(math.Cos(float64(segmentAngle/4)))
			controlX := center + controlRadius*float32(midCos)
			controlY := center + controlRadius*float32(midSin)

			p.QuadTo(f32.Pt(controlX, controlY), f32.Pt(outerEndX, outerEndY))
		} else {
			// For larger segments, draw line (fallback)
			p.LineTo(f32.Pt(outerEndX, outerEndY))
		}

		// Connect to inner arc
		p.LineTo(f32.Pt(innerEndX, innerEndY))

		// Inner arc (reverse direction)
		if segmentAngle <= math.Pi {
			midAngle := endAngle - segmentAngle/2
			midCos, midSin := math.Cos(float64(midAngle)), math.Sin(float64(midAngle))

			controlRadius := innerRadius / float32(math.Cos(float64(segmentAngle/4)))
			controlX := center + controlRadius*float32(midCos)
			controlY := center + controlRadius*float32(midSin)

			p.QuadTo(f32.Pt(controlX, controlY), f32.Pt(innerStartX, innerStartY))
		} else {
			p.LineTo(f32.Pt(innerStartX, innerStartY))
		}

		// Close the path
		p.Close()

		// Fill the segment
		paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	}

	return layout.Dimensions{Size: image.Pt(size, size)}
}

func Timer(th *material.Theme, remaining, total time.Duration) layout.FlexChild {
	progress := 1.0 - float32(remaining.Seconds()/total.Seconds())
	_ = progress
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Dp(unit.Dp(200))
				rect := image.Rect(0, 0, size, size)

				// Outer ring ellipse
				outer := clip.Ellipse{Min: rect.Min, Max: rect.Max}.Op(gtx.Ops)
				paint.FillShape(gtx.Ops, color.NRGBA{R: 0x3D, G: 0x3D, B: 0x3D, A: 0xFF}, outer)

				DrawGradientRing(
					gtx,
					1-float32(remaining.Seconds())/float32(total.Seconds()),
					color.NRGBA{R: 0xF1, G: 0x1D, B: 0x28, A: 0x00}, // start
					color.NRGBA{R: 0xFF, G: 0xA1, B: 0x2C, A: 0xFF}, // end FFA12C
				)
				// Inner circle (cutout effect)
				inset := gtx.Dp(unit.Dp(10))
				innerRect := rect.Inset(inset)
				inner := clip.Ellipse{Min: innerRect.Min, Max: innerRect.Max}.Op(gtx.Ops)
				paint.FillShape(gtx.Ops, color.NRGBA{R: 0x01, G: 0x01, B: 0x01, A: 0xFF}, inner)
				return layout.Dimensions{Size: rect.Size()}

			}),
			// layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			// 	return DrawGradientRing(
			// 		gtx,
			// 		float32(remaining.Seconds())/float32(total.Seconds()),
			// 		color.NRGBA{R: 0x00, G: 0xFF, B: 0x80, A: 0xFF}, // start
			// 		color.NRGBA{R: 0x00, G: 0x80, B: 0xFF, A: 0xFF}, // end
			// 	)
			// }),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,

					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						icon, _ := widget.NewIcon(icons.ActionVisibility)

						iconColor := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
						return icon.Layout(gtx, iconColor)

					}), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						m := material.H3(th, formatDuration(remaining))
						m.Alignment = text.Middle
						m.Color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
						return m.Layout(gtx)

					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								rect := image.Rect(0, 0, 5, 12)
								cRect := clip.UniformRRect(
									rect, 2, // corner radius in pixels
								)
								defer cRect.Push(gtx.Ops).Pop()
								paint.FillShape(gtx.Ops, color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}, cRect.Op(gtx.Ops))
								return layout.Dimensions{Size: cRect.Rect.Size()}
							}), layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {

								rect := clip.UniformRRect(
									image.Rect(0, 0, 5, 12),
									2, // corner radius in pixels
								)
								defer rect.Push(gtx.Ops).Pop()
								// paint.FillShape(gtx.Ops, color.NRGBA{R: 0x01, G: 0x01, B: 0x01, A: 0xFF}, rect.Op(gtx.Ops))
								paint.FillShape(gtx.Ops, color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}, rect.Op(gtx.Ops))
								return layout.Dimensions{Size: rect.Rect.Size()}
							}), layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {

								rect := clip.UniformRRect(
									image.Rect(0, 0, 5, 12),
									2, // corner radius in pixels
								)
								defer rect.Push(gtx.Ops).Pop()
								// paint.FillShape(gtx.Ops, color.NRGBA{R: 0x01, G: 0x01, B: 0x01, A: 0xFF}, rect.Op(gtx.Ops))
								paint.FillShape(gtx.Ops, color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}, rect.Op(gtx.Ops))
								return layout.Dimensions{Size: rect.Rect.Size()}
							}), layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {

								rect := clip.UniformRRect(
									image.Rect(0, 0, 5, 12),
									2, // corner radius in pixels
								)
								defer rect.Push(gtx.Ops).Pop()
								// paint.FillShape(gtx.Ops, color.NRGBA{R: 0x01, G: 0x01, B: 0x01, A: 0xFF}, rect.Op(gtx.Ops))
								paint.FillShape(gtx.Ops, color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF}, rect.Op(gtx.Ops))
								return layout.Dimensions{Size: rect.Rect.Size()}
							}),
						)
					}),
				)
			}))
	})
}
