package argo

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

// Page is one Argo screen. Draw does not load the cluster.
type Page struct {
	ID              string
	Title           string
	Context         string
	Notice          string
	Err             string
	Loading         bool
	AppsLoading     bool
	SetsLoading     bool
	ProjectsLoading bool
	ReposLoading    bool
	ClustersLoading bool
	Apps            []App
	Sets            []Set
	Projects        []Project
	Repos           []Repo
	Clusters        []Cluster
	Activity        []Activity
	Now             time.Time

	Selected *int
	Revision *string
	SyncRev  *string
	Cascade  *string
	Prune    *bool

	OnOpen   func(string)
	OnFleet  func(action string)
	OnAction func(index int, action string)
}

// Draw paints the Argo dashboard or one designed list.
func Draw(c *ui.Context, t *ui.Theme, page Page) kit.Selection {
	if page.Now.IsZero() {
		page.Now = time.Now()
	}
	result := kit.Selection{Index: -1}
	title := page.Title
	if title == "" {
		title = "Argo"
	}
	ui.Scroll(c).Grow(1).FillHeight().Scrollbars(false).Children(func() {
		ui.Column(c).Gap(14).Children(func() {
			header(c, t, title, page)
			if page.Err != "" && !page.Loading {
				ui.Text(c, page.Err).TextColor(t.Danger)
				return
			}
			if page.Notice != "" && !page.Loading {
				ui.Text(c, page.Notice).TextColor(t.TextMuted)
				return
			}
			if page.ID == "argo" || page.ID == "" {
				if page.Loading && !hasRows(page) && !sectionLoading(page) {
					loadingBody(c, t, page.ID)
					return
				}
				if page.Notice != "" {
					ui.Text(c, page.Notice).TextColor(t.TextMuted)
				}
				dashboard(c, t, page)
				return
			}
			if page.Loading && !hasRows(page) {
				loadingBody(c, t, page.ID)
				return
			}
			switch page.ID {
			case "argo", "":
				dashboard(c, t, page)
			case "argo-applications":
				result = applicationList(c, page)
			case "argo-applicationsets":
				result = setList(c, page)
			case "argo-appprojects":
				result = projectList(c, page)
			case "argo-repositories":
				result = repoList(c, page)
			case "argo-clusters":
				result = clusterList(c, page)
			default:
				ui.Text(c, "Not found")
			}
		})
	})
	return result
}

func header(c *ui.Context, t *ui.Theme, title string, page Page) {
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, title).FontSize(22).Bold()
		if page.AppsLoading || (page.Loading && !hasRows(page)) {
			kit.Bar(c, t, 180, 12)
		} else if sub := subtitle(page); sub != "" {
			ui.Text(c, sub).TextColor(t.TextMuted)
		}
	})
}

func sectionLoading(page Page) bool {
	return page.AppsLoading || page.SetsLoading || page.ProjectsLoading || page.ReposLoading || page.ClustersLoading
}

func subtitle(page Page) string {
	name := page.Context
	if name == "" {
		name = "No context"
	}
	switch page.ID {
	case "argo", "":
		return name + "  ·  " + itoa(len(page.Apps)) + " applications"
	case "argo-applications":
		stats := Counts(page.Apps)
		return name + "  ·  " + itoa(stats.OutOfSync) + " out of sync"
	default:
		return name
	}
}

func dashboard(c *ui.Context, t *ui.Theme, page Page) {
	colors := kit.Colors()
	if page.AppsLoading {
		ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
			statSkeleton(c, t, "Applications", colors.Pods)
			statSkeleton(c, t, "Synced", colors.Requests)
			statSkeleton(c, t, "Healthy", colors.Memory)
			statSkeleton(c, t, "Out of sync", colors.Allocatable)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
			statSkeleton(c, t, "Degraded", t.Danger)
			statSkeleton(c, t, "Missing", t.Danger)
			statSkeleton(c, t, "Progressing", colors.CPU)
		})
	} else {
		stats := Counts(page.Apps)
		ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
			statCard(c, t, "Applications", itoa(stats.Total), colors.Pods)
			statCard(c, t, "Synced", itoa(stats.Synced), colors.Requests)
			statCard(c, t, "Healthy", itoa(stats.Healthy), colors.Memory)
			statCard(c, t, "Out of sync", itoa(stats.OutOfSync), colors.Allocatable)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
			statCard(c, t, "Degraded", itoa(stats.Degraded), t.Danger)
			statCard(c, t, "Missing", itoa(stats.Missing), t.Danger)
			statCard(c, t, "Progressing", itoa(stats.Progressing), colors.CPU)
		})
		fleetActions(c, page)
		attention(c, t, page)
	}
	activity(c, t, page)
	ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
		if page.SetsLoading {
			statSkeleton(c, t, "Application sets", t.Text)
		} else {
			linkCard(c, t, "Application sets", itoa(len(page.Sets)), "argo-applicationsets", page.OnOpen)
		}
		if page.ProjectsLoading {
			statSkeleton(c, t, "Projects", t.Text)
		} else {
			linkCard(c, t, "Projects", itoa(len(page.Projects)), "argo-appprojects", page.OnOpen)
		}
		if page.ReposLoading {
			statSkeleton(c, t, "Repositories", t.Text)
		} else {
			linkCard(c, t, "Repositories", itoa(len(page.Repos)), "argo-repositories", page.OnOpen)
		}
		if page.ClustersLoading {
			statSkeleton(c, t, "Clusters", t.Text)
		} else {
			linkCard(c, t, "Clusters", itoa(len(page.Clusters)), "argo-clusters", page.OnOpen)
		}
	})
}

func statCard(c *ui.Context, t *ui.Theme, title, value string, color ui.Color) {
	kit.Panel(c, t).Grow(1).Children(func() {
		kit.Title(c, t, nil, title, color)
		ui.Text(c, value).FontSize(32).Bold()
	})
}

func detailStat(c *ui.Context, t *ui.Theme, title, value string, color ui.Color) {
	ui.Column(c).Grow(1).MinWidth(120).Padding(10, 12).Gap(3).Radius(8).Background(kit.Tint(t, color)).Children(func() {
		ui.Text(c, title).FontSize(11).Bold().TextColor(color)
		ui.Text(c, value).FontSize(14).Bold().SingleLine()
	})
}

func fleetActions(c *ui.Context, page Page) {
	if page.OnFleet == nil || len(page.Apps) == 0 {
		return
	}
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Sync").Clicked() {
			page.OnFleet("sync-drift")
		}
		if ui.Button(c, "Refresh").Clicked() {
			page.OnFleet("refresh-all")
		}
	})
}

func linkCard(c *ui.Context, t *ui.Theme, title, value, id string, onOpen func(string)) {
	kit.Panel(c, t).Grow(1).Children(func() {
		kit.Title(c, t, nil, title, t.Text)
		ui.Text(c, value).FontSize(32).Bold()
		if onOpen != nil && ui.Button(c, "Open").Clicked() {
			onOpen(id)
		}
	})
}

func attention(c *ui.Context, t *ui.Theme, page Page) {
	rows := NeedsAttention(page.Apps)
	kit.Panel(c, t).Children(func() {
		ui.Text(c, "Needs attention").FontSize(16).Bold()
		if len(rows) == 0 {
			ui.Text(c, "All Applications are synced and healthy.").TextColor(t.TextMuted)
			return
		}
		for _, row := range rows {
			reason, object, body := row.Reason, row.Namespace+"/"+row.Name, "Sync: "+row.Sync+" · Health: "+row.Health
			color := t.Danger
			if reason == "Out of sync" {
				color = kit.Colors().Allocatable
			}
			ui.Column(c).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(color)
					ui.Text(c, reason).Bold().Width(120)
					ui.Text(c, object).Grow(1).MinWidth(0)
				})
				ui.Text(c, body).TextColor(t.TextMuted)
			})
		}
	})
}

func activity(c *ui.Context, t *ui.Theme, page Page) {
	kit.Panel(c, t).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Recent activity").FontSize(16).Bold()
			ui.Text(c, itoa(len(page.Activity))).TextColor(t.TextMuted)
		})
		if len(page.Activity) == 0 {
			ui.Text(c, "No Argo events").TextColor(t.TextMuted)
			return
		}
		shown := page.Activity
		if len(shown) > 8 {
			shown = shown[:8]
		}
		for _, row := range shown {
			reason, object, body, ageText, category := row.Reason, row.Object, row.Body, age(row.When, page.Now), row.Category
			ui.Column(c).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(activityColor(t, category))
					ui.Text(c, reason).Bold().Width(140)
					ui.Text(c, object).Grow(1).MinWidth(0)
					if ageText != "" {
						ui.Text(c, ageText).TextColor(t.TextMuted).Shrink(0)
					}
				})
				if body != "" {
					ui.Text(c, body).TextColor(t.TextMuted)
				}
			})
		}
	})
}

func activityColor(t *ui.Theme, category string) ui.Color {
	switch category {
	case "failure":
		return t.Danger
	case "success":
		return kit.Colors().Requests
	case "progress":
		return kit.Colors().CPU
	default:
		return t.TextMuted
	}
}

func applicationList(c *ui.Context, page Page) kit.Selection {
	if len(page.Apps) == 0 {
		ui.Text(c, "No applications")
		return kit.Selection{Index: -1}
	}
	index := selectedIndex(page.Selected, len(page.Apps))
	table := ui.Table(c, &ui.ListState{Selected: &index}, appColumns(), len(page.Apps), func(i, col int) {
		ui.Text(c, appCell(page.Apps[i], col))
	})
	if page.Selected != nil {
		*page.Selected = index
	}
	return kit.Selection{Index: index, Valid: index >= 0, Changed: table.Changed()}
}

func setList(c *ui.Context, page Page) kit.Selection {
	if len(page.Sets) == 0 {
		ui.Text(c, "No application sets")
		return kit.Selection{Index: -1}
	}
	index := selectedIndex(page.Selected, len(page.Sets))
	table := ui.Table(c, &ui.ListState{Selected: &index}, []ui.TableColumn{
		{Title: "Namespace", Width: 140},
		{Title: "Name", Width: 180},
		{Title: "Generators", Width: 180},
		{Title: "Project", Width: 140},
	}, len(page.Sets), func(i, col int) {
		item := page.Sets[i]
		cells := []string{item.Namespace, item.Name, item.Generators, item.Project}
		ui.Text(c, cells[col])
	})
	if page.Selected != nil {
		*page.Selected = index
	}
	return kit.Selection{Index: index, Valid: index >= 0, Changed: table.Changed()}
}

func projectList(c *ui.Context, page Page) kit.Selection {
	if len(page.Projects) == 0 {
		ui.Text(c, "No projects")
		return kit.Selection{Index: -1}
	}
	index := selectedIndex(page.Selected, len(page.Projects))
	table := ui.Table(c, &ui.ListState{Selected: &index}, []ui.TableColumn{
		{Title: "Namespace", Width: 140},
		{Title: "Name", Width: 180},
		{Title: "Description", Width: 220},
		{Title: "Source repos", Width: 110},
	}, len(page.Projects), func(i, col int) {
		item := page.Projects[i]
		cells := []string{item.Namespace, item.Name, item.Description, itoa(len(item.SourceRepos))}
		ui.Text(c, cells[col])
	})
	if page.Selected != nil {
		*page.Selected = index
	}
	return kit.Selection{Index: index, Valid: index >= 0, Changed: table.Changed()}
}

func repoList(c *ui.Context, page Page) kit.Selection {
	if len(page.Repos) == 0 {
		ui.Text(c, "No repositories")
		return kit.Selection{Index: -1}
	}
	index := selectedIndex(page.Selected, len(page.Repos))
	table := ui.Table(c, &ui.ListState{Selected: &index}, []ui.TableColumn{
		{Title: "Name", Width: 160},
		{Title: "Type", Width: 90},
		{Title: "URL", Width: 280},
		{Title: "Project", Width: 140},
	}, len(page.Repos), func(i, col int) {
		item := page.Repos[i]
		cells := []string{item.Name, item.Type, item.URL, item.Project}
		ui.Text(c, cells[col])
	})
	if page.Selected != nil {
		*page.Selected = index
	}
	return kit.Selection{Index: index, Valid: index >= 0, Changed: table.Changed()}
}

func clusterList(c *ui.Context, page Page) kit.Selection {
	if len(page.Clusters) == 0 {
		ui.Text(c, "No cluster secrets. The in-cluster destination has none.")
		return kit.Selection{Index: -1}
	}
	index := selectedIndex(page.Selected, len(page.Clusters))
	table := ui.Table(c, &ui.ListState{Selected: &index}, []ui.TableColumn{
		{Title: "Name", Width: 180},
		{Title: "Server", Width: 320},
	}, len(page.Clusters), func(i, col int) {
		item := page.Clusters[i]
		cells := []string{item.Name, item.Server}
		ui.Text(c, cells[col])
	})
	if page.Selected != nil {
		*page.Selected = index
	}
	return kit.Selection{Index: index, Valid: index >= 0, Changed: table.Changed()}
}

func DetailTitle(page Page, index int) string {
	switch page.ID {
	case "argo-applications":
		if index >= 0 && index < len(page.Apps) {
			return page.Apps[index].Namespace + "/" + page.Apps[index].Name
		}
	case "argo-applicationsets":
		if index >= 0 && index < len(page.Sets) {
			return page.Sets[index].Namespace + "/" + page.Sets[index].Name
		}
	case "argo-appprojects":
		if index >= 0 && index < len(page.Projects) {
			return page.Projects[index].Namespace + "/" + page.Projects[index].Name
		}
	case "argo-repositories":
		if index >= 0 && index < len(page.Repos) {
			return page.Repos[index].Name
		}
	case "argo-clusters":
		if index >= 0 && index < len(page.Clusters) {
			return page.Clusters[index].Name
		}
	}
	return ""
}

// HeaderActions returns resource actions supported by the shared detail header.
func HeaderActions(page Page, index int) (edit, remove string) {
	if page.OnAction == nil || DetailTitle(page, index) == "" {
		return "", ""
	}
	switch page.ID {
	case "argo-applications", "argo-applicationsets", "argo-appprojects":
		return "yaml", "delete"
	default:
		return "", ""
	}
}

func Detail(c *ui.Context, t *ui.Theme, page Page, index int) {
	switch page.ID {
	case "argo-applications":
		if index >= 0 && index < len(page.Apps) {
			detailApp(c, t, page, page.Apps[index], index)
		}
	case "argo-applicationsets":
		if index >= 0 && index < len(page.Sets) {
			item := page.Sets[index]
			statLine(c, t, "Generators", orDash(item.Generators))
			statLine(c, t, "Project", orDash(item.Project))
			statLine(c, t, "Destination", orDash(item.Destination))
			lines(c, t, "Conditions", item.Conditions)
		}
	case "argo-appprojects":
		if index >= 0 && index < len(page.Projects) {
			item := page.Projects[index]
			statLine(c, t, "Description", orDash(item.Description))
			lines(c, t, "Source repositories", item.SourceRepos)
			lines(c, t, "Destinations", item.Destinations)
			statLine(c, t, "Roles", itoa(item.Roles))
		}
	case "argo-repositories":
		if index >= 0 && index < len(page.Repos) {
			item := page.Repos[index]
			statLine(c, t, "Type", orDash(item.Type))
			statLine(c, t, "URL", orDash(item.URL))
			statLine(c, t, "Project", orDash(item.Project))
			ui.Text(c, "Credentials stay on the Secret.").TextColor(t.TextMuted)
		}
	case "argo-clusters":
		if index >= 0 && index < len(page.Clusters) {
			item := page.Clusters[index]
			statLine(c, t, "Server", orDash(item.Server))
			ui.Text(c, "Credentials stay on the Secret.").TextColor(t.TextMuted)
		}
	}
}

func detailApp(c *ui.Context, t *ui.Theme, page Page, app App, index int) {
	ui.Row(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
		detailStat(c, t, "Sync", shown(app.Sync), syncColor(t, app.Sync))
		detailStat(c, t, "Health", shown(app.Health), healthColor(t, app.Health))
	})
	if app.HealthMessage != "" {
		ui.Text(c, app.HealthMessage).TextColor(t.TextMuted)
	}
	ui.Row(c).Gap(12).Wrap().Children(func() {
		mini(c, t, "Project", orDash(app.Project))
		mini(c, t, "Destination", orDash(app.Destination))
		mini(c, t, "Revision", orDash(ShortRevision(app.Revision)))
		mini(c, t, "Policy", orDash(app.Policy))
	})
	statLine(c, t, "Source", orDash(app.Source))
	if app.SourceType != "" {
		statLine(c, t, "Source type", app.SourceType)
	}
	if len(app.Images) > 0 {
		lines(c, t, "Images", app.Images)
	}
	lines(c, t, "Conditions", app.Conditions)
	resources(c, t, app.Resources)
	history(c, t, app.History)
	op := app.OperationPhase
	if app.OperationMessage != "" {
		op = join(op, app.OperationMessage)
	}
	if app.OperationBy != "" {
		op = join(op, app.OperationBy)
	}
	statLine(c, t, "Operation", orDash(op))
	controls(c, page)
	actions(c, index, []string{"sync", "refresh", "hard-refresh", "rollback"}, page.OnAction)
}

func resources(c *ui.Context, t *ui.Theme, rows []Resource) {
	ui.Text(c, "Resources").Bold()
	if len(rows) == 0 {
		ui.Text(c, "None").TextColor(t.TextMuted)
		return
	}
	for _, row := range rows {
		kind, name, status := row.Kind, row.Name, shown(row.Sync)+" · "+shown(row.Health)
		if row.Hook {
			kind += " hook"
		}
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, kind).Width(120)
			ui.Text(c, name).Grow(1).MinWidth(0)
			ui.Text(c, status).TextColor(t.TextMuted).Shrink(0)
		})
	}
}

func history(c *ui.Context, t *ui.Theme, rows []History) {
	ui.Text(c, "History").Bold()
	if len(rows) == 0 {
		ui.Text(c, "None").TextColor(t.TextMuted)
		return
	}
	for _, row := range rows {
		id, rev, by := itoa(int(row.ID)), ShortRevision(row.Revision), orDash(row.By)
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, id).Width(40)
			ui.Text(c, rev).Width(90)
			ui.Text(c, by).TextColor(t.TextMuted)
		})
	}
}

func controls(c *ui.Context, page Page) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		if page.SyncRev != nil {
			ui.TextInput(c, page.SyncRev).Label("Sync revision").Placeholder("target").Width(140)
		}
		if page.Revision != nil {
			ui.TextInput(c, page.Revision).Label("Rollback id").Width(80)
		}
		if page.Prune != nil {
			labels := []string{"Keep", "Prune"}
			index := 0
			if *page.Prune {
				index = 1
			}
			ui.Segmented(c, &index, labels...).Label("Prune")
			*page.Prune = index == 1
		}
		if page.Cascade != nil {
			labels := []string{"Foreground", "Background", "Non-cascading"}
			values := []string{"foreground", "background", "non-cascading"}
			index := 0
			for i, value := range values {
				if *page.Cascade == value {
					index = i
				}
			}
			ui.Segmented(c, &index, labels...).Label("Cascade")
			if index >= 0 && index < len(values) {
				*page.Cascade = values[index]
			}
		}
	})
}

func actions(c *ui.Context, index int, names []string, on func(int, string)) {
	ui.Row(c).Gap(8).Wrap().Children(func() {
		for _, name := range names {
			label := actionLabel(name)
			if ui.Button(c, label).Clicked() && on != nil {
				on(index, name)
			}
		}
	})
}

func actionLabel(action string) string {
	switch action {
	case "sync":
		return "Sync"
	case "refresh":
		return "Refresh"
	case "hard-refresh":
		return "Hard refresh"
	case "rollback":
		return "Rollback"
	case "yaml":
		return "YAML"
	case "delete":
		return "Delete"
	default:
		return action
	}
}

func appColumns() []ui.TableColumn {
	return []ui.TableColumn{
		{Title: "Namespace", Width: 140},
		{Title: "Name", Width: 160},
		{Title: "Sync", Width: 100},
		{Title: "Health", Width: 100},
		{Title: "Project", Width: 120},
		{Title: "Destination", Width: 180},
		{Title: "Revision", Width: 90},
	}
}

func appCell(app App, col int) string {
	cells := []string{
		app.Namespace,
		app.Name,
		shown(app.Sync),
		shown(app.Health),
		app.Project,
		app.Destination,
		ShortRevision(app.Revision),
	}
	if col < 0 || col >= len(cells) {
		return ""
	}
	return cells[col]
}

func syncColor(t *ui.Theme, status string) ui.Color {
	switch status {
	case "Synced":
		return kit.Colors().Requests
	case "OutOfSync":
		return kit.Colors().Allocatable
	default:
		return t.TextMuted
	}
}

func healthColor(t *ui.Theme, status string) ui.Color {
	switch status {
	case "Healthy":
		return kit.Colors().Memory
	case "Progressing":
		return kit.Colors().CPU
	case "Degraded", "Missing":
		return t.Danger
	case "Suspended":
		return kit.Colors().Allocatable
	default:
		return t.TextMuted
	}
}

func mini(c *ui.Context, t *ui.Theme, name, value string) {
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, name).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, value).Bold()
	})
}

func statLine(c *ui.Context, t *ui.Theme, name, value string) {
	ui.Row(c).Gap(8).Children(func() {
		ui.Text(c, name).TextColor(t.TextMuted).Width(120)
		ui.Text(c, value).Grow(1).MinWidth(0)
	})
}

func lines(c *ui.Context, t *ui.Theme, name string, rows []string) {
	ui.Text(c, name).Bold()
	if len(rows) == 0 {
		ui.Text(c, "None").TextColor(t.TextMuted)
		return
	}
	for _, row := range rows {
		text := row
		ui.Text(c, text).TextColor(t.TextMuted)
	}
}

func hasRows(page Page) bool {
	return len(page.Apps)+len(page.Sets)+len(page.Projects)+len(page.Repos)+len(page.Clusters) > 0
}

func selectedIndex(selected *int, n int) int {
	if selected == nil || *selected < 0 || *selected >= n {
		return -1
	}
	return *selected
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func join(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " · " + b
}

func age(when, now time.Time) string {
	if when.IsZero() {
		return ""
	}
	d := now.Sub(when)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return itoa(int(d.Seconds())) + "s"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + "h"
	default:
		return itoa(int(d.Hours()/24)) + "d"
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func loadingBody(c *ui.Context, t *ui.Theme, id string) {
	kit.Status(c, "Loading").Children(func() {
		if id == "argo" || id == "" {
			dashboardSkeleton(c, t)
			return
		}
		listSkeleton(c, t, listTitles(id))
	})
}

func dashboardSkeleton(c *ui.Context, t *ui.Theme) {
	colors := kit.Colors()
	ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
		statSkeleton(c, t, "Applications", colors.Pods)
		statSkeleton(c, t, "Synced", colors.Requests)
		statSkeleton(c, t, "Healthy", colors.Memory)
		statSkeleton(c, t, "Out of sync", colors.Allocatable)
	})
	ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
		statSkeleton(c, t, "Degraded", t.Danger)
		statSkeleton(c, t, "Missing", t.Danger)
		statSkeleton(c, t, "Progressing", colors.CPU)
	})
	kit.Panel(c, t).Children(func() {
		ui.Text(c, "Needs attention").FontSize(16).Bold()
		for range 3 {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(t.Border)
				kit.Bar(c, t, 120, 12)
				kit.Bar(c, t, 0, 12)
			})
		}
	})
	kit.Panel(c, t).Children(func() {
		ui.Text(c, "Recent activity").FontSize(16).Bold()
		for range 3 {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(t.Border)
				kit.Bar(c, t, 140, 12)
				kit.Bar(c, t, 0, 12)
				kit.Bar(c, t, 36, 12)
			})
		}
	})
	ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
		statSkeleton(c, t, "Application sets", t.Text)
		statSkeleton(c, t, "Projects", t.Text)
		statSkeleton(c, t, "Repositories", t.Text)
		statSkeleton(c, t, "Clusters", t.Text)
	})
}

func statSkeleton(c *ui.Context, t *ui.Theme, title string, color ui.Color) {
	kit.Panel(c, t).Grow(1).MinWidth(0).Children(func() {
		kit.Title(c, t, nil, title, color)
		kit.Bar(c, t, 48, 28)
	})
}

func listSkeleton(c *ui.Context, t *ui.Theme, titles []string) {
	row := func(header bool) {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			for i, title := range titles {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					if header {
						ui.Text(c, title).FontSize(12).TextColor(t.TextMuted).NoWrap()
						return
					}
					width := float32(72)
					if i == 1 || i == 5 {
						width = 120
					}
					kit.Bar(c, t, width, 12)
				})
			}
		})
	}
	row(true)
	for range 6 {
		row(false)
	}
}

func listTitles(id string) []string {
	switch id {
	case "argo-applications":
		return []string{"Namespace", "Name", "Sync", "Health", "Project", "Destination", "Revision"}
	case "argo-applicationsets":
		return []string{"Namespace", "Name", "Generators", "Project"}
	case "argo-appprojects":
		return []string{"Namespace", "Name", "Description", "Source repos"}
	case "argo-repositories":
		return []string{"Name", "Type", "URL", "Project"}
	case "argo-clusters":
		return []string{"Name", "Server"}
	default:
		return []string{"Namespace", "Name"}
	}
}
