package desktop

import "strings"

// shortContextName trims the provider noise from generated context names so
// the expanded rail shows what people call the cluster. The full name stays
// in the tooltip and label.
//
//	arn:aws:eks:ap-southeast-1:123456789012:cluster/prod -> prod
//	gke_project_zone_name                                -> name
func shortContextName(name string) string {
	if strings.HasPrefix(name, "arn:") {
		if i := strings.LastIndex(name, "/"); i >= 0 && i < len(name)-1 {
			return name[i+1:]
		}
	}
	if strings.HasPrefix(name, "gke_") {
		if parts := strings.SplitN(name, "_", 4); len(parts) == 4 && parts[3] != "" {
			return parts[3]
		}
	}
	return name
}
