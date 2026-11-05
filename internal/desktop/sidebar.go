package desktop

// Item is one sidebar row. ID is stable; the GVR map in a later phase
// covers only the generic kinds.
type Item struct {
	ID    string
	Label string
}

// Section is one sidebar group. Resource diagram and Flux are not here.
type Section struct {
	Title string
	Items []Item
}

// Sections is the locked sidebar. Overview, workloads home, Helm, Argo,
// Istio, and port-forwards are screens of their own.
func Sections() []Section {
	return []Section{
		{Title: "Cluster", Items: []Item{
			{ID: "overview", Label: "Overview"},
			{ID: "nodes", Label: "Nodes"},
			{ID: "namespaces", Label: "Namespaces"},
			{ID: "events", Label: "Events"},
			{ID: "leases", Label: "Leases"},
			{ID: "port-forwards", Label: "Port-forwards"},
		}},
		{Title: "Workloads", Items: []Item{
			{ID: "workloads", Label: "Workloads"},
			{ID: "pods", Label: "Pods"},
			{ID: "deployments", Label: "Deployments"},
			{ID: "replicasets", Label: "ReplicaSets"},
			{ID: "daemonsets", Label: "DaemonSets"},
			{ID: "statefulsets", Label: "StatefulSets"},
			{ID: "jobs", Label: "Jobs"},
			{ID: "cronjobs", Label: "CronJobs"},
		}},
		{Title: "Networking", Items: []Item{
			{ID: "services", Label: "Services"},
			{ID: "ingresses", Label: "Ingresses"},
			{ID: "endpointslices", Label: "EndpointSlices"},
			{ID: "networkpolicies", Label: "NetworkPolicies"},
			{ID: "ingressclasses", Label: "IngressClasses"},
		}},
		{Title: "Configuration", Items: []Item{
			{ID: "configmaps", Label: "ConfigMaps"},
			{ID: "secrets", Label: "Secrets"},
			{ID: "horizontalpodautoscalers", Label: "HPAs"},
			{ID: "limitranges", Label: "LimitRanges"},
			{ID: "resourcequotas", Label: "ResourceQuotas"},
			{ID: "poddisruptionbudgets", Label: "PodDisruptionBudgets"},
		}},
		{Title: "Storage", Items: []Item{
			{ID: "persistentvolumes", Label: "PersistentVolumes"},
			{ID: "persistentvolumeclaims", Label: "PersistentVolumeClaims"},
			{ID: "volumeattachments", Label: "VolumeAttachments"},
			{ID: "storageclasses", Label: "StorageClasses"},
			{ID: "csidrivers", Label: "CSIDrivers"},
			{ID: "csinodes", Label: "CSINodes"},
		}},
		{Title: "Access", Items: []Item{
			{ID: "serviceaccounts", Label: "ServiceAccounts"},
			{ID: "roles", Label: "Roles"},
			{ID: "rolebindings", Label: "RoleBindings"},
			{ID: "clusterroles", Label: "ClusterRoles"},
			{ID: "clusterrolebindings", Label: "ClusterRoleBindings"},
		}},
		{Title: "Administration", Items: []Item{
			{ID: "crds", Label: "CRDs"},
			{ID: "priorityclasses", Label: "PriorityClasses"},
			{ID: "runtimeclasses", Label: "RuntimeClasses"},
			{ID: "mutatingwebhookconfigurations", Label: "MutatingWebhookConfigurations"},
			{ID: "validatingwebhookconfigurations", Label: "ValidatingWebhookConfigurations"},
			{ID: "validatingadmissionpolicies", Label: "ValidatingAdmissionPolicies"},
			{ID: "validatingadmissionpolicybindings", Label: "ValidatingAdmissionPolicyBindings"},
		}},
		{Title: "Helm", Items: []Item{
			{ID: "helm", Label: "Helm"},
		}},
		{Title: "Argo", Items: []Item{
			{ID: "argo", Label: "Dashboard"},
			{ID: "argo-applications", Label: "Applications"},
			{ID: "argo-applicationsets", Label: "ApplicationSets"},
			{ID: "argo-appprojects", Label: "AppProjects"},
			{ID: "argo-repositories", Label: "Repositories"},
			{ID: "argo-clusters", Label: "Clusters"},
		}},
		{Title: "Istio", Items: []Item{
			{ID: "istio-virtualservices", Label: "VirtualServices"},
			{ID: "istio-destinationrules", Label: "DestinationRules"},
			{ID: "istio-gateways", Label: "Gateways"},
			{ID: "istio-serviceentries", Label: "ServiceEntries"},
			{ID: "istio-sidecars", Label: "Sidecars"},
			{ID: "istio-peerauthentications", Label: "PeerAuthentications"},
			{ID: "istio-authorizationpolicies", Label: "AuthorizationPolicies"},
			{ID: "istio-requestauthentications", Label: "RequestAuthentications"},
		}},
	}
}

// SidebarIDs lists every sidebar id. resource-diagram and flux are absent.
func SidebarIDs() []string {
	var ids []string
	for _, section := range Sections() {
		for _, item := range section.Items {
			ids = append(ids, item.ID)
		}
	}
	return ids
}
