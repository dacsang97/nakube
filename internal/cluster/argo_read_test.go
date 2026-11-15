package cluster

import (
	"encoding/base64"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseArgoApp(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "shop", "namespace": "argo"},
		"spec": map[string]any{
			"project": "default",
			"source": map[string]any{
				"repoURL":        "https://operator:s3cret-token@git.example/shop",
				"path":           "deploy",
				"targetRevision": "main",
			},
			"destination": map[string]any{"name": "in-cluster", "namespace": "shop"},
			"syncPolicy":  map[string]any{"automated": map[string]any{"prune": true, "selfHeal": true}},
		},
		"status": map[string]any{
			"sync":   map[string]any{"status": "OutOfSync", "revision": "abcdef1234567890"},
			"health": map[string]any{"status": "Degraded", "message": "rollout failed"},
			"resources": []any{
				map[string]any{"kind": "Deployment", "namespace": "shop", "name": "web", "status": "OutOfSync", "health": map[string]any{"status": "Degraded"}},
			},
			"history": []any{
				map[string]any{"id": float64(2), "revision": "bbbbbbbbbbbb", "initiatedBy": map[string]any{"username": "nakube"}},
				map[string]any{"id": float64(9), "revision": "cccccccccccc", "initiatedBy": map[string]any{"automated": true}},
			},
			"conditions": []any{map[string]any{"type": "ComparisonError", "message": "denied"}},
			"operationState": map[string]any{
				"phase":   "Failed",
				"message": "sync error",
				"operation": map[string]any{
					"initiatedBy": map[string]any{"username": "nakube"},
				},
			},
			"sourceType": "Helm",
		},
	}}
	app := ParseArgoApp(obj)
	if app.Name != "shop" || app.Sync != "OutOfSync" || app.Health != "Degraded" {
		t.Fatalf("%+v", app)
	}
	if app.Destination != "in-cluster/shop" || app.TargetRevision != "main" || app.Policy != "Automated, prune, self-heal" {
		t.Fatalf("spec %+v", app)
	}
	if app.Source != "https://git.example/shop deploy main" {
		t.Fatalf("source %q", app.Source)
	}
	if len(app.Sources) != 1 || app.Sources[0].RepoURL != "https://git.example/shop" ||
		strings.Contains(app.Source+app.Sources[0].RepoURL, "operator") ||
		strings.Contains(app.Source+app.Sources[0].RepoURL, "s3cret-token") {
		t.Fatalf("source credentials leaked: %+v", app)
	}
	if len(app.Resources) != 1 || app.Resources[0].Health != "Degraded" {
		t.Fatalf("resources %+v", app.Resources)
	}
	if len(app.History) != 2 || app.History[0].ID != 9 || app.History[0].By != "automated" {
		t.Fatalf("history %+v", app.History)
	}
	if len(app.Conditions) != 1 || app.Conditions[0] != "ComparisonError: denied" {
		t.Fatalf("conditions %+v", app.Conditions)
	}
	if app.OperationPhase != "Failed" || app.OperationBy != "nakube" || app.SourceType != "Helm" {
		t.Fatalf("operation %+v", app)
	}
}

func TestParseArgoSourcesPreferList(t *testing.T) {
	app := ParseArgoApp(&unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"source":  map[string]any{"repoURL": "https://ignored.example"},
			"sources": []any{map[string]any{"repoURL": "git:s3cret@git.example:a", "path": "one"}},
		},
	}})
	if strings.Contains(app.Source, "ignored") || strings.Contains(app.Source, "s3cret") ||
		app.Source != "git.example:a one" || app.TargetRevision != "" ||
		len(app.Sources) != 1 || app.Sources[0].RepoURL != "git.example:a" {
		t.Fatalf("%+v", app)
	}
}

func TestParseArgoSetAndProject(t *testing.T) {
	set := ParseArgoApplicationSet(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "shops", "namespace": "argo"},
		"spec": map[string]any{
			"generators": []any{
				map[string]any{"git": map[string]any{"repoURL": "https://git.example"}},
				map[string]any{"list": map[string]any{}},
			},
			"template": map[string]any{"spec": map[string]any{
				"project":     "default",
				"destination": map[string]any{"server": "https://set-user:set-secret@kubernetes.default.svc", "namespace": "shop"},
			}},
		},
	}})
	if set.Generators != "git, list" || set.Project != "default" ||
		set.Destination != "https://kubernetes.default.svc/shop" ||
		strings.Contains(set.Destination, "set-user") || strings.Contains(set.Destination, "set-secret") {
		t.Fatalf("%+v", set)
	}
	project := ParseArgoProject(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "default", "namespace": "argo"},
		"spec": map[string]any{
			"description": "apps",
			"sourceRepos": []any{
				"https://project-user:project-secret@git.example/shop",
				"git:deploy-secret@git.example:team/repo",
				"deploy@git.example:team/public",
				"git.example/ordinary/path",
			},
			"destinations": []any{
				map[string]any{"server": "https://cluster-user:cluster-secret@prod.example", "namespace": "shop"},
			},
			"roles": []any{map[string]any{"name": "read"}},
		},
	}})
	if project.Description != "apps" || project.Roles != 1 ||
		len(project.SourceRepos) != 4 || project.SourceRepos[0] != "https://git.example/shop" ||
		project.SourceRepos[1] != "git.example:team/repo" ||
		project.SourceRepos[2] != "git.example:team/public" ||
		project.SourceRepos[3] != "git.example/ordinary/path" ||
		len(project.Destinations) != 1 || project.Destinations[0] != "https://prod.example/shop" {
		t.Fatalf("%+v", project)
	}
	combined := strings.Join(project.SourceRepos, " ") + strings.Join(project.Destinations, " ")
	for _, secret := range []string{"project-user", "project-secret", "deploy-secret", "cluster-user", "cluster-secret"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("project credential %q leaked in %+v", secret, project)
		}
	}
}

func TestParseArgoSecretDropsCredentials(t *testing.T) {
	secret := "s3cret-token"
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "repo", "namespace": "argo"},
		"data": map[string]any{
			"url":           base64.StdEncoding.EncodeToString([]byte("https://operator:s3cret-token@git.example/shop")),
			"type":          base64.StdEncoding.EncodeToString([]byte("git")),
			"name":          base64.StdEncoding.EncodeToString([]byte("shop")),
			"project":       base64.StdEncoding.EncodeToString([]byte("default")),
			"password":      base64.StdEncoding.EncodeToString([]byte(secret)),
			"sshPrivateKey": base64.StdEncoding.EncodeToString([]byte(secret)),
		},
	}}
	repo := ParseArgoRepository(obj)
	got := repo.Name + repo.URL + repo.Type + repo.Project
	if strings.Contains(got, secret) || strings.Contains(got, "operator") ||
		repo.URL != "https://git.example/shop" || repo.Type != "git" {
		t.Fatalf("%+v", repo)
	}
	cluster := ParseArgoCluster(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "prod"},
		"data": map[string]any{
			"name":   base64.StdEncoding.EncodeToString([]byte("prod")),
			"server": base64.StdEncoding.EncodeToString([]byte("https://cluster-user:s3cret-token@prod.example")),
			"config": base64.StdEncoding.EncodeToString([]byte(secret)),
		},
	}})
	if cluster.Server != "https://prod.example" ||
		strings.Contains(cluster.Name+cluster.Server, secret) ||
		strings.Contains(cluster.Server, "cluster-user") {
		t.Fatalf("%+v", cluster)
	}
}

func TestBuildArgoRefreshPatch(t *testing.T) {
	data, err := BuildArgoRefreshPatch("hard")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"argocd.argoproj.io/refresh":"hard"`) {
		t.Fatalf("%s", data)
	}
	if _, err := BuildArgoRefreshPatch("soft"); err == nil {
		t.Fatal("invalid refresh accepted")
	}
}
