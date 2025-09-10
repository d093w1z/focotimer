package widgets

import (
	"image/color"

	"github.com/d093w1z/gio/layout"
	"github.com/d093w1z/gio/unit"
	"github.com/d093w1z/gio/widget"
	"github.com/d093w1z/gio/widget/material"
)

func Button(th *material.Theme, inset unit.Dp, label string, icon []byte, btnWidget *widget.Clickable, onClick func()) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {

		startIcon, _ := widget.NewIcon(icon)
		btn := material.IconButton(th, btnWidget, startIcon, label)
		btn.Background = color.NRGBA{R: 0x3D, G: 0x3D, B: 0x3D, A: 0xFF}
		btn.Inset = layout.UniformInset(inset)
		if btnWidget.Clicked(gtx) {
			onClick()
		}
		return btn.Layout(gtx)
	})
}
