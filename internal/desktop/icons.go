package desktop

import (
	"embed"
	"path"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Icons are Lucide 0.544.0, ISC. The notice is in icons/LICENSE.
// The renderer paints them with the text color, so the stroke stays theme-aware.

//go:embed icons/*.svg
var iconFiles embed.FS

var iconSet = map[string]*ui.SVG{}

func init() {
	entries, err := iconFiles.ReadDir("icons")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".svg") {
			continue
		}
		body, err := iconFiles.ReadFile(path.Join("icons", name))
		if err != nil {
			panic(err)
		}
		iconSet[strings.TrimSuffix(name, ".svg")] = ui.MustParseSVG(body)
	}
}

func IconNamed(name string) *ui.SVG {
	return iconSet[name]
}

func IconFor(id string) *ui.SVG {
	name, ok := iconByID[id]
	if !ok {
		return iconSet["box"]
	}
	return iconSet[name]
}

var iconByID = map[string]string{
	"overview":      "layout-dashboard",
	"nodes":         "server",
	"namespaces":    "boxes",
	"events":        "bell",
	"leases":        "timer",
	"port-forwards": "arrow-left-right",

	"workloads":    "layers",
	"pods":         "box",
	"deployments":  "rocket",
	"replicasets":  "copy",
	"daemonsets":   "cpu",
	"statefulsets": "database",
	"jobs":         "play",
	"cronjobs":     "clock",

	"services":        "network",
	"ingresses":       "globe",
	"endpointslices":  "locate",
	"networkpolicies": "shield",
	"ingressclasses":  "list",

	"configmaps":               "file-cog",
	"secrets":                  "key-round",
	"horizontalpodautoscalers": "gauge",
	"limitranges":              "gauge",
	"resourcequotas":           "chart-pie",
	"poddisruptionbudgets":     "shield-alert",

	"persistentvolumes":      "hard-drive",
	"persistentvolumeclaims": "hard-drive",
	"volumeattachments":      "paperclip",
	"storageclasses":         "archive",
	"csidrivers":             "cpu",
	"csinodes":               "server",

	"serviceaccounts":     "user",
	"roles":               "shield",
	"rolebindings":        "link",
	"clusterroles":        "shield",
	"clusterrolebindings": "link",

	"crds":                              "puzzle",
	"priorityclasses":                   "list-ordered",
	"runtimeclasses":                    "terminal",
	"mutatingwebhookconfigurations":     "webhook",
	"validatingwebhookconfigurations":   "webhook",
	"validatingadmissionpolicies":       "scale",
	"validatingadmissionpolicybindings": "scale",

	"helm": "package",

	"argo":                 "gauge",
	"argo-applications":    "git-branch",
	"argo-applicationsets": "layers",
	"argo-appprojects":     "folder-sync",
	"argo-repositories":    "database",
	"argo-clusters":        "globe",

	"istio-virtualservices":        "route",
	"istio-destinationrules":       "split",
	"istio-gateways":               "door-open",
	"istio-serviceentries":         "globe",
	"istio-sidecars":               "box",
	"istio-peerauthentications":    "shield",
	"istio-authorizationpolicies":  "shield-alert",
	"istio-requestauthentications": "key-round",
	"settings":                     "settings",
}
