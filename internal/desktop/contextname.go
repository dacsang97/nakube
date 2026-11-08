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

// contextDisplayNames keeps provider-generated names compact when that is
// unambiguous and adds stable provider details when two contexts share the
// same short name.
func contextDisplayNames(names []string) []string {
	labels := make([]string, len(names))
	counts := make(map[string]int, len(names))
	for i, name := range names {
		labels[i] = shortContextName(name)
		counts[labels[i]]++
	}
	for i, name := range names {
		if counts[labels[i]] > 1 {
			labels[i] = disambiguatedContextName(name, labels[i])
		}
	}
	return labels
}

func disambiguatedContextName(name, short string) string {
	if strings.HasPrefix(name, "arn:") {
		parts := strings.SplitN(name, ":", 6)
		if len(parts) == 6 && parts[3] != "" && parts[4] != "" {
			return short + " · " + parts[3] + " · " + parts[4]
		}
	}
	if strings.HasPrefix(name, "gke_") {
		parts := strings.SplitN(name, "_", 4)
		if len(parts) == 4 && parts[1] != "" && parts[2] != "" {
			return short + " · " + parts[1] + " · " + parts[2]
		}
	}
	return name
}
