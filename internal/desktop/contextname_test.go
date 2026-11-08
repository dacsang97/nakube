package desktop

import "testing"

func TestShortContextName(t *testing.T) {
	cases := map[string]string{
		"arn:aws:eks:ap-southeast-1:123456789012:cluster/prod": "prod",
		"arn:aws:eks:us-east-1:1:cluster/":                     "arn:aws:eks:us-east-1:1:cluster/",
		"gke_my-project_us-central1-a_staging":                 "staging",
		"gke_my-project_us-central1-a_name_with_underscores":   "name_with_underscores",
		"gke_broken": "gke_broken",
		"kind-local": "kind-local",
		"":           "",
	}
	for in, want := range cases {
		if got := shortContextName(in); got != want {
			t.Errorf("shortContextName(%q) = %q, want %q", in, got, want)
		}
	}
}
