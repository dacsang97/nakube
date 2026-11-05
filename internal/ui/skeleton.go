package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Bar is one text-sized placeholder. A zero width grows. The pulse is the
// documented placeholder; reduced motion holds a steady opacity. Either way
// the next frame is requested so a waiting screen can replace the bar.
func Bar(c *ui.Context, t *ui.Theme, width, height float32) ui.Element {
	if height <= 0 {
		height = 12
	}
	radius := height / 2
	if radius > 6 {
		radius = 6
	}
	return pulse(c, t, width, height, radius, width <= 0)
}

// Block is a chart or editor well. It grows across the row.
func Block(c *ui.Context, t *ui.Theme, height, radius float32) ui.Element {
	if height <= 0 {
		height = 12
	}
	if radius < 0 {
		radius = 0
	}
	return pulse(c, t, 0, height, radius, true)
}

// Circle is a donut-sized placeholder.
func Circle(c *ui.Context, t *ui.Theme, size float32) ui.Element {
	if size <= 0 {
		size = 96
	}
	return pulse(c, t, size, size, size/2, false)
}

// Status is the busy region a skeleton stands in. Label is what assistive
// technology reads; it is not painted.
func Status(c *ui.Context, label string) ui.Element {
	if c != nil {
		c.AnimationFrame()
	}
	if label == "" {
		label = "Loading"
	}
	return ui.Column(c).Role(ui.RoleStatus).Label(label).Gap(10).Grow(1).MinWidth(0)
}

func pulse(c *ui.Context, t *ui.Theme, width, height, radius float32, grow bool) ui.Element {
	if c != nil {
		c.AnimationFrame()
	}
	var fill ui.Color
	if t != nil {
		fill = t.Border
	}
	e := ui.Box(c).Height(height).Radius(radius).Background(fill).Shrink(0)
	if grow {
		e.Grow(1).MinWidth(0)
	} else if width > 0 {
		e.Width(width)
	}
	opacity := float32(0.72)
	if c != nil && !c.Preferences().ReduceMotion {
		opacity = 0.4 + 0.6*e.Loop("pulse", 1600*time.Millisecond, ui.Bounce(ui.EaseInOut))
	}
	return e.Opacity(opacity)
}
