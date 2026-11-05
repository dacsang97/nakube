package cluster

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// IstioDestination is one weighted destination on a route rule.
type IstioDestination struct {
	Host   string
	Subset string
	Port   int64
	Weight int64
}

// IstioRule is one http, tls, or tcp item. Weights sit on its destinations.
type IstioRule struct {
	Match        string
	Destinations []IstioDestination
}

// ExtractRouteRules returns one rule per spec.http, spec.tls, or spec.tcp item.
func ExtractRouteRules(obj map[string]any, field string) []IstioRule {
	items, _, _ := unstructured.NestedSlice(obj, "spec", field)
	rules := make([]IstioRule, 0, len(items))
	for _, item := range items {
		rule, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rules = append(rules, IstioRule{
			Match:        summariseMatch(rule["match"]),
			Destinations: destinations(rule["route"]),
		})
	}
	return rules
}

func destinations(raw any) []IstioDestination {
	items, ok := raw.([]any)
	if !ok {
		return []IstioDestination{}
	}
	out := make([]IstioDestination, 0, len(items))
	for _, item := range items {
		route, ok := item.(map[string]any)
		if !ok {
			continue
		}
		dest, _ := route["destination"].(map[string]any)
		host, _ := dest["host"].(string)
		subset, _ := dest["subset"].(string)
		var port int64
		if p, ok := dest["port"].(map[string]any); ok {
			port = toInt64(p["number"])
		}
		out = append(out, IstioDestination{
			Host:   host,
			Subset: subset,
			Port:   port,
			Weight: toInt64(route["weight"]),
		})
	}
	return out
}

func summariseMatch(raw any) string {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return "any"
	}
	var parts []string
	for _, item := range items {
		match, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if uri, ok := match["uri"].(map[string]any); ok {
			for kind, value := range uri {
				parts = append(parts, fmt.Sprintf("uri %s %v", kind, value))
			}
		}
		if method, ok := match["method"].(map[string]any); ok {
			for kind, value := range method {
				parts = append(parts, fmt.Sprintf("method %s %v", kind, value))
			}
		}
	}
	if len(parts) == 0 {
		return "any"
	}
	return strings.Join(parts, ", ")
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		return 0
	}
}

// IstioKinds are the eight designed kinds. Version comes from the served CRD.
var IstioKinds = []struct {
	ID    string
	Group string
	Kind  string
}{
	{"istio-virtualservices", "networking.istio.io", "VirtualService"},
	{"istio-destinationrules", "networking.istio.io", "DestinationRule"},
	{"istio-gateways", "networking.istio.io", "Gateway"},
	{"istio-serviceentries", "networking.istio.io", "ServiceEntry"},
	{"istio-sidecars", "networking.istio.io", "Sidecar"},
	{"istio-peerauthentications", "security.istio.io", "PeerAuthentication"},
	{"istio-authorizationpolicies", "security.istio.io", "AuthorizationPolicy"},
	{"istio-requestauthentications", "security.istio.io", "RequestAuthentication"},
}
