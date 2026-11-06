package desktop

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

const (
	// Connected tab: 35px row, 8px cap. The inverse shoulder is only
	// beside another tab. The first tab sits on the editor edge with no left S.
	tabStripH = 35
	tabMinW   = 80
	tabMaxW   = 160
	tabCap    = 8
	shoulder  = 8
)

// showScreen opens screen from a sidebar click. ephemeral
// replaces the preview tab. A screen that is already open is focused, and
// a non-ephemeral show pins it.
func (a *App) showScreen(screen string, ephemeral bool) {
	a.ensureTabs()
	if a.tabs.Screen(a.active) == screen {
		if !ephemeral {
			a.tabs.Pin(a.active)
		}
		a.syncRouter(screen)
		return
	}
	if id, ok := a.tabs.byScreen(screen); ok {
		if !ephemeral {
			a.tabs.Pin(id)
		}
		a.activate(id)
		return
	}
	a.stash()
	id, _, closed := a.tabs.Show(screen, ephemeral)
	a.mu.Lock()
	if closed != "" {
		dead := a.bodies[closed]
		delete(a.bodies, closed)
		a.mu.Unlock()
		releaseBody(dead)
		a.mu.Lock()
	}
	if a.bodies == nil {
		a.bodies = map[string]*tabBody{}
	}
	body := newTabBody()
	if inspectScreen(screen) {
		body.row = -1
	}
	a.bodies[id] = body
	a.active = id
	a.mu.Unlock()
	a.restore()
	a.syncRouter(screen)
	if !isLive(screen) {
		a.loadList(screen)
	}
	a.invalidate()
}

// bindCurrent attaches the fields already on App to a new tab of screen.
// Tests and a replaced router set the screen before the strip exists.
func (a *App) bindCurrent(screen string) {
	a.ensureTabs()
	id, _, closed := a.tabs.Show(screen, true)
	if inspectScreen(screen) && a.listKey == "" {
		a.row = -1
	}
	b := a.capture()
	b.loaded = true
	b.listID = screen
	a.mu.Lock()
	if closed != "" {
		dead := a.bodies[closed]
		delete(a.bodies, closed)
		a.mu.Unlock()
		releaseBody(dead)
		a.mu.Lock()
	}
	a.bodies[id] = b
	a.active = id
	a.selected = screen
	a.mu.Unlock()
}

func (a *App) activate(id string) {
	if id == "" {
		return
	}
	a.ensureTabs()
	if a.tabs.tab(id) == nil {
		return
	}
	if a.active == id {
		a.tabs.Select(id)
		if d := a.tabs.Detail(id); d != nil {
			a.mu.Lock()
			if d.parent != "" {
				a.selected = d.parent
			}
			a.mu.Unlock()
			return
		}
		screen := a.tabs.Screen(id)
		a.mu.Lock()
		a.selected = screen
		a.mu.Unlock()
		a.syncRouter(screen)
		return
	}
	a.stash()
	a.tabs.Select(id)
	a.mu.Lock()
	a.active = id
	if a.bodies[id] == nil {
		a.bodies[id] = newTabBody()
	}
	a.mu.Unlock()
	a.restore()
	a.finishDetailOrScreen(id)
	a.invalidate()
}

func (a *App) finishDetailOrScreen(id string) {
	if d := a.tabs.Detail(id); d != nil {
		a.mu.Lock()
		if d.parent != "" {
			a.selected = d.parent
		}
		a.mu.Unlock()
		return
	}
	screen := a.tabs.Screen(id)
	a.mu.Lock()
	a.selected = screen
	a.mu.Unlock()
	a.syncRouter(screen)
	if !isLive(screen) && !a.bodyLoaded(id) {
		a.loadList(screen)
	}
}

func (a *App) bodyLoaded(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.bodies[id]
	return b != nil && b.loaded
}

func (a *App) closeTab(id string) {
	a.ensureTabs()
	if a.tabs.tab(id) == nil {
		return
	}
	was := a.active == id
	if was {
		a.stash()
	}
	a.mu.Lock()
	dead := a.bodies[id]
	delete(a.bodies, id)
	if was {
		a.active = ""
		a.logCancel = nil
		a.execTerm = nil
		a.execConn = nil
	}
	a.mu.Unlock()
	releaseBody(dead)
	next := a.tabs.Close(id)
	if next == "" {
		var closed string
		next, _, closed = a.tabs.Show("overview", false)
		if closed != "" {
			a.dropBody(closed)
		}
		a.mu.Lock()
		if a.bodies[next] == nil {
			a.bodies[next] = newTabBody()
		}
		a.mu.Unlock()
	}
	a.activate(next)
}

func (a *App) closeOtherTabs(id string) {
	a.ensureTabs()
	if a.tabs.tab(id) == nil {
		return
	}
	if a.active != id {
		a.activate(id)
	}
	for _, dead := range a.tabs.CloseOthers(id) {
		a.dropBody(dead)
	}
	a.invalidate()
}

func (a *App) closeTabsRight(id string) {
	a.ensureTabs()
	dead := a.tabs.CloseRight(id)
	closedActive := false
	for _, id := range dead {
		if id == a.active {
			closedActive = true
		}
		a.dropBody(id)
	}
	if closedActive {
		a.mu.Lock()
		a.active = ""
		a.logCancel = nil
		a.execTerm = nil
		a.execConn = nil
		a.mu.Unlock()
		a.activate(a.tabs.Selected())
	}
	a.invalidate()
}

func (a *App) closeAllTabs() {
	a.ensureTabs()
	a.stash()
	dead := a.tabs.Clear()
	a.mu.Lock()
	a.active = ""
	a.logCancel = nil
	a.execTerm = nil
	a.execConn = nil
	a.mu.Unlock()
	for _, id := range dead {
		a.dropBody(id)
	}
	id, _, _ := a.tabs.Show("overview", false)
	a.mu.Lock()
	a.bodies[id] = newTabBody()
	a.active = id
	a.mu.Unlock()
	a.restore()
	a.syncRouter("overview")
	a.invalidate()
}

func (a *App) dropBody(id string) {
	a.mu.Lock()
	dead := a.bodies[id]
	delete(a.bodies, id)
	if a.active == id {
		a.active = ""
		a.logCancel = nil
		a.execTerm = nil
		a.execConn = nil
	}
	a.mu.Unlock()
	releaseBody(dead)
}

func (a *App) followRouter() {
	if a.router == nil {
		a.router = ui.NewRouter("/overview")
	}
	a.ensureTabs()
	if a.tabs.Detail(a.active) != nil {
		return
	}
	screen := screenID(a.router.Path())
	if a.tabs.Screen(a.active) == screen {
		return
	}
	if _, ok := a.tabs.byScreen(screen); ok {
		a.showScreen(screen, true)
		return
	}
	if a.selected == screen {
		a.bindCurrent(screen)
		return
	}
	a.showScreen(screen, true)
}

func (a *App) syncRouter(screen string) {
	if a.router == nil {
		a.router = ui.NewRouter(pagePath(screen))
		return
	}
	if screenID(a.router.Path()) != screen {
		a.router.Replace(pagePath(screen))
	}
}

func (a *App) tabKeys(e ui.Element) {
	nextMods, nextKey := ui.Ctrl, ui.KeyPageUp
	prevMods, prevKey := ui.Ctrl, ui.KeyPageDown
	if runtime.GOOS == "darwin" {
		nextMods, nextKey = ui.Cmd|ui.Alt, ui.KeyRight
		prevMods, prevKey = ui.Cmd|ui.Alt, ui.KeyLeft
	}
	switch {
	case e.Shortcut(ui.Cmd, ui.KeyW):
		a.closeTab(a.tabs.Selected())
	case e.Shortcut(nextMods, nextKey):
		a.activate(a.tabs.Next())
	case e.Shortcut(prevMods, prevKey):
		a.activate(a.tabs.Prev())
	case e.Shortcut(ui.Cmd, ui.Key1):
		a.activate(a.tabs.At(0))
	case e.Shortcut(ui.Cmd, ui.Key2):
		a.activate(a.tabs.At(1))
	case e.Shortcut(ui.Cmd, ui.Key3):
		a.activate(a.tabs.At(2))
	case e.Shortcut(ui.Cmd, ui.Key4):
		a.activate(a.tabs.At(3))
	case e.Shortcut(ui.Cmd, ui.Key5):
		a.activate(a.tabs.At(4))
	case e.Shortcut(ui.Cmd, ui.Key6):
		a.activate(a.tabs.At(5))
	case e.Shortcut(ui.Cmd, ui.Key7):
		a.activate(a.tabs.At(6))
	case e.Shortcut(ui.Cmd, ui.Key8):
		a.activate(a.tabs.At(7))
	case e.Shortcut(ui.Cmd, ui.Key9):
		a.activate(a.tabs.At(-1))
	}
}

func (a *App) drawTabs(c *ui.Context, t *ui.Theme) {
	a.ensureTabs()
	ui.Row(c).Height(tabStripH).FillWidth().Shrink(0).Background(tabInactive(t)).Children(func() {
		ui.ScrollHorizontal(c).FillWidth().FillHeight().Children(func() {
			ui.Row(c).FillHeight().Gap(shoulder).Children(func() {
				vis := a.tabs.Visible()
				for i, tab := range vis {
					a.drawTab(c, t, tab, i == 0)
				}
			})
		})
	})
}

func (a *App) drawTab(c *ui.Context, t *ui.Theme, tab *mainTab, first bool) {
	on := tab.id == a.tabs.Selected()
	row := ui.Row(c.Key("tab-"+tab.id)).MinWidth(tabMinW).MaxWidth(tabMaxW).FillHeight().Shrink(0).Padding(0, 8).Gap(4).AlignItems(ui.Center)
	if on {
		// The outer tab is square on the editor edge. The S is only inward.
		if first {
			row.Margin(4, 0, 0, 0).Radius(0, tabCap, 0, 0)
		} else {
			row.Margin(4, 0, 0, 0).Radius(tabCap, tabCap, 0, 0)
		}
		row.Background(editorCard(t)).ScrollIntoView()
	} else {
		row.Margin(4, 0)
		if first {
			row.Radius(0, 6, 6, 0)
		} else {
			row.Radius(6)
		}
		if row.Hovered() {
			row.Background(t.SurfacePressed)
		}
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("Close").Chosen() {
			a.closeTab(tab.id)
		}
		m.Separator()
		if m.Item("Close other tabs").Disabled(a.tabs.Len() < 2).Chosen() {
			a.closeOtherTabs(tab.id)
		}
		if m.Item("Close tabs to the right").Disabled(!a.tabs.HasRight(tab.id)).Chosen() {
			a.closeTabsRight(tab.id)
		}
		if m.Item("Close all").Chosen() {
			a.closeAllTabs()
		}
	})
	var closed bool
	showClose := on || row.Hovered()
	row.Children(func() {
		if on && !first {
			smoothShoulder(c, editorCard(t), true)
		}
		if on {
			smoothShoulder(c, editorCard(t), false)
		}
		icon := t.TextMuted
		if on {
			icon = t.Text
		}
		ui.Icon(c, tabGlyph(tab)).Size(16, 16).Shrink(0).TextColor(icon)
		label := ui.Text(c, tabLabel(tab)).FontSize(13).SingleLine().MinWidth(0).Grow(1)
		if tab.ephemeral {
			label.Italic()
		}
		if on {
			label.TextColor(t.Text)
		} else {
			label.TextColor(t.TextMuted)
		}
		closeBtn := ui.Button(c, "").Size(16, 16).Padding(0).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Tooltip("Close")
		closeBtn.Border(0, ui.Transparent).TextColor(t.TextMuted)
		if closeBtn.Hovered() {
			closeBtn.Background(t.SurfacePressed).TextColor(t.Text)
		}
		if showClose {
			closeBtn.Children(func() {
				ui.Text(c, "×").FontSize(14)
			})
		}
		if closeBtn.Clicked() {
			closed = true
			a.closeTab(tab.id)
		}
	})
	if row.DoubleClicked() {
		a.tabs.Pin(tab.id)
	} else if !closed && row.Clicked() {
		a.activate(tab.id)
	}
	a.dragTab(row, tab.id)
}

// smoothShoulder is the checked corner: one quarter-circle from the vertical
// tab side to the horizontal editor edge. The tangent matches both, so there
// is no extra step.
func smoothShoulder(c *ui.Context, color ui.Color, left bool) {
	box := ui.Box(c).Absolute().Bottom(0).Width(shoulder).Height(shoulder)
	if left {
		box.Left(-shoulder)
	} else {
		box.Right(-shoulder)
	}
	box.DrawOver(func(p *ui.Painter, r ui.Rect) {
		const k = float32(0.5522847498)
		rad := r.W
		path := &ui.Path{}
		if left {
			// Circle centered at the outer top. Vertical tangent on the tab side,
			// horizontal tangent on the editor line. Fill the tab-corner side.
			path.MoveTo(r.X+r.W, r.Y)
			path.CubeTo(r.X+r.W, r.Y+k*rad, r.X+k*rad, r.Y+r.H, r.X, r.Y+r.H)
			path.LineTo(r.X+r.W, r.Y+r.H)
		} else {
			path.MoveTo(r.X, r.Y)
			path.CubeTo(r.X, r.Y+k*rad, r.X+r.W-k*rad, r.Y+r.H, r.X+r.W, r.Y+r.H)
			path.LineTo(r.X, r.Y+r.H)
		}
		path.Close()
		p.FillPath(path, color)
	})
}

func tabInactive(t *ui.Theme) ui.Color {
	if t.Dark {
		return t.SurfaceHover
	}
	return t.Surface
}

func tabIcon(screen string) *ui.SVG {
	if screen == "catalog" {
		return IconNamed("plus")
	}
	return IconFor(screen)
}

func (a *App) openDetail(kind, objectKind, namespace, name, title string) {
	a.ensureTabs()
	parent := a.tabs.Screen(a.active)
	if parent == "" {
		if d := a.tabs.Detail(a.active); d != nil {
			parent = d.parent
		}
	}
	if parent == "" {
		parent = a.selected
	}
	if parent == "" {
		parent = "overview"
	}
	key := kind + "\x00" + objectKind + "/" + namespace + "/" + name
	id, created := a.tabs.ShowDetail(kind, key, title, parent)
	if created {
		a.mu.Lock()
		if a.bodies == nil {
			a.bodies = map[string]*tabBody{}
		}
		if a.bodies[id] == nil {
			a.bodies[id] = newTabBody()
		}
		a.mu.Unlock()
	}
	a.activate(id)
}

func (a *App) activeDetail() *tabDetail {
	if a.tabs == nil {
		return nil
	}
	return a.tabs.Detail(a.active)
}

func (a *App) dragTab(row ui.Element, id string) {
	dx, _, held := row.Dragged()
	if !held {
		if a.dragID == id {
			a.dragID = ""
			a.dragAcc = 0
		}
		return
	}
	if a.dragID != id {
		a.dragID = id
		a.dragAcc = 0
	}
	a.dragAcc += dx
	width := row.Bounds().W
	if width < 40 {
		width = tabMinW
	}
	shift := int(a.dragAcc / width)
	if shift == 0 {
		return
	}
	vis := a.tabs.Visible()
	from := -1
	for i, t := range vis {
		if t.id == id {
			from = i
			break
		}
	}
	if from < 0 {
		return
	}
	to := from + shift
	if to < 0 {
		to = 0
	}
	if to >= len(vis) {
		to = len(vis) - 1
	}
	if to == from || !a.tabs.Move(id, to) {
		return
	}
	a.dragAcc -= float32(to-from) * width
}

func tabLabel(tab *mainTab) string {
	if tab.detail != nil && tab.detail.title != "" {
		return tab.detail.title
	}
	return labelFor(tab.screen)
}

func tabGlyph(tab *mainTab) *ui.SVG {
	if tab.detail != nil {
		switch tab.detail.kind {
		case "logs":
			return IconNamed("list")
		case "exec":
			return IconNamed("terminal")
		case "yaml":
			return IconNamed("file-cog")
		}
	}
	return tabIcon(tab.screen)
}
