package cluster

import (
	"os"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestExtractRouteRulesOneRowPerRule(t *testing.T) {
	body, err := os.ReadFile("testdata/istio/virtualservice.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := yaml.Unmarshal(body, &obj); err != nil {
		t.Fatal(err)
	}
	rules := ExtractRouteRules(obj, "http")
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
	if len(rules[0].Destinations) != 2 {
		t.Fatalf("destinations = %+v", rules[0].Destinations)
	}
	if rules[0].Destinations[0].Weight != 90 || rules[0].Destinations[1].Weight != 10 {
		t.Fatalf("weights = %+v", rules[0].Destinations)
	}
	if rules[0].Destinations[0].Host != "reviews" || rules[0].Destinations[1].Subset != "v2" {
		t.Fatalf("destinations = %+v", rules[0].Destinations)
	}
}

func TestIstioKindCount(t *testing.T) {
	if len(IstioKinds) != 8 {
		t.Fatalf("kinds = %d", len(IstioKinds))
	}
}
