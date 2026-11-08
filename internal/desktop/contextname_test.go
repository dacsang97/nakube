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

func TestContextDisplayNamesDisambiguatesDuplicateShortNames(t *testing.T) {
	names := []string{
		"arn:aws:eks:ap-southeast-1:111111111111:cluster/prod",
		"arn:aws:eks:us-east-1:222222222222:cluster/prod",
		"gke_project-a_us-central1-a_staging",
		"gke_project-b_europe-west1-b_staging",
		"kind-local",
	}
	want := []string{
		"prod · ap-southeast-1 · 111111111111",
		"prod · us-east-1 · 222222222222",
		"staging · project-a · us-central1-a",
		"staging · project-b · europe-west1-b",
		"kind-local",
	}
	got := contextDisplayNames(names)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("contextDisplayNames(%q) = %q, want %q", names[i], got[i], want[i])
		}
	}
}
