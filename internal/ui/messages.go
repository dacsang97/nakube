package ui

import "github.com/egoist/mygo/ui"

// Message is one warning row. Body is the event message.
type Message struct {
	Reason string
	Object string
	Body   string
	Age    string
}

// Messages is the issues list: a count, then reason, object, and the message.
func Messages(c *ui.Context, t *ui.Theme, count int, rows []Message, onOpen func()) {
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, "Messages").FontSize(16).Bold()
		ui.Text(c, "Warnings: "+itoa(count)).TextColor(t.TextMuted).Grow(1)
		if onOpen != nil && ui.Button(c, "Events").Clicked() {
			onOpen()
		}
	})
	if len(rows) == 0 {
		ui.Text(c, "No issues found").TextColor(t.TextMuted)
		return
	}
	for _, row := range rows {
		reason, object, body, age := row.Reason, row.Object, row.Body, row.Age
		ui.Column(c).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(t.Danger)
				ui.Text(c, reason).Bold().Width(120)
				ui.Text(c, object).Grow(1).MinWidth(0)
				if age != "" {
					ui.Text(c, age).TextColor(t.TextMuted).Shrink(0)
				}
			})
			if body != "" {
				ui.Text(c, body).TextColor(t.TextMuted)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
