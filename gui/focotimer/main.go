package main

import (
	"flag"
	"image"
	"image/color"
	"log"
	"sync"
	"time"

	focotimer "github.com/d093w1z/focotimer/api"
	"github.com/d093w1z/focotimer/gui/focotimer/polybar"
	widgets "github.com/d093w1z/focotimer/gui/focotimer/widgets"
	"github.com/d093w1z/gio/app"
	"github.com/d093w1z/gio/io/event"
	"github.com/d093w1z/gio/io/key"
	"github.com/d093w1z/gio/io/system"
	"github.com/d093w1z/gio/layout"
	"github.com/d093w1z/gio/op"
	"github.com/d093w1z/gio/op/clip"
	"github.com/d093w1z/gio/op/paint"
	"github.com/d093w1z/gio/unit"
	"github.com/d093w1z/gio/widget"
	"github.com/d093w1z/gio/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type C = layout.Context
type D = layout.Dimensions

var isPolybarEnabled = flag.Bool("polybar", false, "Enable polybar output")

var lastRemaining time.Duration
var lastRemainingMu sync.RWMutex

type Page int64

const (
	TimerStopped Page = iota
	TimerRunning
	TimerFinished
	Splash
	Settings
)

var (
	btnStartStop      = new(widget.Clickable)
	btnPause          = new(widget.Clickable)
	btnIncrease       = new(widget.Clickable)
	btnDecrease       = new(widget.Clickable)
	btnSettings       = new(widget.Clickable)
	btnBack           = new(widget.Clickable)
	page         Page = TimerStopped
)

type AppManager struct {
	window *app.Window
	mu     sync.Mutex
}

// Start creates the window and launches the event loop
func (m *AppManager) Start() {
	m.mu.Lock()
	if m.window != nil {
		m.mu.Unlock()
		return
	}

	m.window = new(app.Window)
	m.window.Option(app.Decorated(false), app.Transparent(true), app.Size(300, 300), app.Title("Pomodoro Timer"))
	m.mu.Unlock()

	go func() {
		if err := m.loop(m.window); err != nil {
			log.Fatal(err)
		}
	}()
}

// Stop closes the window safely
func (m *AppManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.window != nil {
		m.window.Invalidate()
		m.window.Perform(system.ActionClose)
		m.window = nil
	}
}

// ToggleState starts/stops the GUI window
func (m *AppManager) ToggleState() {
	m.mu.Lock()
	windowRunning := m.window != nil
	m.mu.Unlock()

	if !windowRunning {
		go m.Start()
	} else {
		go m.Stop()
	}
}

func getLastRemaining() time.Duration {
	return focotimer.GTimerManager.Snapshot()
}

// ---------------- GUI LOOP ----------------
func (m *AppManager) loop(window *app.Window) error {
	var ops op.Ops
	th := material.NewTheme()

	for {
		e := window.Event()
		switch e := e.(type) {
		case app.DestroyEvent:
			m.mu.Lock()
			if m.window == window {
				m.window = nil
			}
			m.mu.Unlock()
			return e.Err

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			// Key input handling
			event.Op(gtx.Ops, window)
			key.InputHintOp{Tag: window, Hint: key.HintAny}.Add(gtx.Ops)
			for {
				ev, ok := gtx.Source.Event(key.Filter{Focus: nil})
				if !ok {
					break
				}
				if keyEv, ok := ev.(key.Event); ok && keyEv.Name == key.NameEscape && keyEv.State == key.Press {
					m.Stop()
				}
			}

			// Draw rounded background
			rect := clip.UniformRRect(
				image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y),
				8,
			)
			rect.Push(gtx.Ops)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0x01, G: 0x01, B: 0x01, A: 0xFF}, rect.Op(gtx.Ops))

			timerPage(th, gtx, getLastRemaining())

			gtx.Execute(op.InvalidateCmd{}) // refresh
			e.Frame(gtx.Ops)
		}
	}
}

// ---------------- TIMER PAGE ----------------
func timerPage(th *material.Theme, gtx C, remaining time.Duration) D {
	var mainIcon []byte
	if page == TimerRunning {
		mainIcon = icons.AVLoop
	} else {
		mainIcon = icons.AVPlayArrow
	}

	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			widgets.Timer(th, remaining, focotimer.GTimerManager.Timer.Duration),
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			layout.Rigid(func(gtx C) D {
				inset := layout.UniformInset(unit.Dp(8))
				return inset.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						widgets.Button(th, 10, "BACK", icons.NavigationArrowBack, btnBack, func() { page = TimerStopped }),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						widgets.Button(th, 5, "DECREASE", icons.ContentRemove, btnDecrease, func() {
							focotimer.GTimerManager.Dec()
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						widgets.Button(th, 10, "PLAY/PAUSE", mainIcon, btnStartStop, func() {
							if page == TimerRunning {
								page = TimerStopped
								focotimer.GTimerManager.Stop()
								focotimer.GTimerManager.Reset()

							} else {
								page = TimerRunning

								focotimer.GTimerManager.Reset()
								focotimer.GTimerManager.Start()
								go func() {
									<-focotimer.GTimerManager.Done()
									page = TimerFinished
								}()
							}
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						widgets.Button(th, 5, "INCREASE", icons.ContentAdd, btnIncrease, func() {
							focotimer.GTimerManager.Inc()
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						widgets.Button(th, 10, "SETTINGS", icons.ActionSettings, btnSettings, func() {
							page = Settings
							focotimer.GTimerManager.Stop()
						}),
					)
				})
			}),
		)
	})
}

// ---------------- MAIN ----------------
func main() {
	manager := &AppManager{}

	flag.Parse()
	if *isPolybarEnabled {
		polybar.Init()
		polybar.SetTimerManager(focotimer.GTimerManager)
		polybar.AddHandler(manager.ToggleState)
		go polybar.Main()
	} else {
		manager.Start()
	}

	app.Main()
}
