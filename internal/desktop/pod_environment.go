package desktop

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dacsang97/nakube/internal/cluster"
)

type podEnvMutator func(context.Context, string, cluster.Identity, string, bool, string, *string) error

// podEnvironmentTarget follows rollout owners rather than attempting the
// immutable Pod update. ReplicaSets and Jobs are intermediate owners.
func podEnvironmentTarget(pod *corev1.Pod, objects map[string]runtime.Object) (cluster.Identity, bool) {
	if pod == nil {
		return cluster.Identity{}, false
	}
	ref := metav1.GetControllerOf(pod)
	seen := map[string]bool{}
	for ref != nil {
		key := ref.Kind + "/" + pod.Namespace + "/" + ref.Name
		if seen[key] {
			return cluster.Identity{}, false
		}
		seen[key] = true
		switch ref.Kind {
		case "Deployment", "StatefulSet", "DaemonSet", "CronJob":
			return cluster.Identity{Kind: ref.Kind, Namespace: pod.Namespace, Name: ref.Name}, true
		case "ReplicaSet", "Job":
			obj := objects[key]
			owner, err := apiMeta.Accessor(obj)
			if err != nil {
				return cluster.Identity{}, false
			}
			ref = metav1.GetControllerOf(owner)
		default:
			return cluster.Identity{}, false
		}
	}
	return cluster.Identity{}, false
}

func (a *App) bindPodEnvironment(key string) {
	if a.podEnvPod == key {
		return
	}
	a.podEnvGen++
	a.podEnvPod = key
	a.podEnvContainer = ""
	a.podEnvInit = false
	a.podEnvName = ""
	a.podEnvValue = ""
	a.podEnvEditing = ""
	a.podEnvErr = ""
	a.podEnvContext = a.current
}

func (a *App) drawPodEnvironment(c *ui.Context, t *ui.Theme, pod *corev1.Pod, target cluster.Identity, editable bool) {
	if pod == nil {
		return
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Text(c, "Environment").FontSize(14).FontWeight(650).Grow(1).MinWidth(140)
			if !editable {
				ui.Badge(c, "Read-only")
			}
		})
		if editable {
			ui.Text(c, fmt.Sprintf("Edits %s/%s · rolls Pods", target.Kind, target.Name)).
				FontSize(12).TextColor(t.TextMuted).Wrap()
		} else {
			ui.Text(c, "Live Pod environment is immutable · no supported rollout owner").
				FontSize(12).TextColor(t.TextMuted).Wrap()
		}
		if a.podEnvSwitching {
			ui.Text(c, "Changing context…").FontSize(12).TextColor(t.TextMuted)
		} else if a.podEnvBusy {
			ui.Text(c, "Saving environment change…").FontSize(12).TextColor(t.TextMuted)
		}
		if a.podEnvErr != "" {
			ui.Text(c, "Environment change failed: "+a.podEnvErr).FontSize(12).TextColor(t.Danger).Wrap()
		}
		if len(pod.Spec.InitContainers) == 0 && len(pod.Spec.Containers) == 0 {
			ui.Text(c, "No containers expose environment settings.").FontSize(12).TextColor(t.TextMuted)
		}
		for i := range pod.Spec.InitContainers {
			a.drawContainerEnvironment(c, t, pod.Spec.InitContainers[i], true, target, editable)
		}
		for i := range pod.Spec.Containers {
			a.drawContainerEnvironment(c, t, pod.Spec.Containers[i], false, target, editable)
		}
	})
}

func (a *App) drawContainerEnvironment(c *ui.Context, t *ui.Theme, container corev1.Container, init bool, target cluster.Identity, editable bool) {
	kind := "App"
	if init {
		kind = "Init"
	}
	count := len(container.Env) + len(container.EnvFrom)
	countLabel := fmt.Sprintf("%d sources", count)
	if count == 1 {
		countLabel = "1 source"
	}
	active := a.podEnvContainer == container.Name && a.podEnvInit == init
	cardLabel := fmt.Sprintf("%s container environment, %s", container.Name, countLabel)
	ui.Column(c).Gap(0).Radius(6).Clip().Background(t.Surface).Border(1, t.Border).Label(cardLabel).Children(func() {
		ui.Row(c).MinHeight(40).Padding(6, 8).Gap(6).AlignItems(ui.Center).Background(t.SurfaceHover).Children(func() {
			ui.Text(c, container.Name).FontSize(12).FontWeight(650).SingleLine().Tooltip(container.Name).
				Grow(1).MinWidth(0)
			ui.Badge(c, kind).Shrink(0)
			ui.Badge(c, countLabel).Shrink(0)
			if editable && !active {
				add := panelHeaderButton(c, t, "Add environment variable to "+container.Name, "plus", false).
					Disabled(a.podEnvBusy || a.podEnvSwitching)
				if add.Clicked() {
					a.podEnvContainer, a.podEnvInit = container.Name, init
					a.podEnvName, a.podEnvValue, a.podEnvEditing, a.podEnvErr = "", "", "", ""
				}
			}
		})
		ui.Column(c).Gap(0).Padding(0, 8, 8, 8).Children(func() {
			if count == 0 {
				ui.Text(c, "No environment sources configured.").Padding(8, 0, 0, 0).
					FontSize(12).TextColor(t.TextMuted)
			}
			for _, env := range container.Env {
				env := env
				ui.Row(c).MinHeight(38).Gap(6).AlignItems(ui.Center).
					BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
						ui.Text(c, env.Name).FontSize(12).FontWeight(600).SingleLine().Tooltip(env.Name)
						ui.Text(c, podEnvValue(env)).FontSize(12).TextColor(t.TextMuted).SingleLine().
							Tooltip(podEnvValue(env))
					})
					if editable && env.ValueFrom == nil {
						edit := panelHeaderButton(c, t, "Edit "+env.Name+" in "+container.Name, "pencil", false).
							Disabled(a.podEnvBusy || a.podEnvSwitching)
						if edit.Clicked() {
							a.podEnvContainer, a.podEnvInit, a.podEnvEditing = container.Name, init, env.Name
							a.podEnvName, a.podEnvValue, a.podEnvErr = env.Name, env.Value, ""
						}
					}
					if editable {
						remove := panelHeaderButton(c, t, "Remove "+env.Name+" from "+container.Name, "trash-2", true).
							Disabled(a.podEnvBusy || a.podEnvSwitching)
						if remove.Clicked() {
							a.changePodEnvironment(target, container.Name, init, env.Name, nil)
						}
					}
				})
			}
			for i, source := range container.EnvFrom {
				source := source
				ui.Row(c.Key(fmt.Sprintf("env-from-%d", i))).MinHeight(38).Gap(6).AlignItems(ui.Center).
					BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
						ui.Text(c, "envFrom").FontSize(12).FontWeight(600)
						ui.Text(c, podEnvFromValue(source)).FontSize(12).TextColor(t.TextMuted).SingleLine().
							Tooltip(podEnvFromValue(source))
					})
					ui.Badge(c, "Source").Shrink(0)
				})
			}
			if editable && active {
				a.drawPodEnvironmentForm(c, t, target, container.Name, init)
			}
		})
	})
}

func (a *App) drawPodEnvironmentForm(c *ui.Context, t *ui.Theme, target cluster.Identity, container string, init bool) {
	ui.Column(c).Gap(8).Padding(10).Radius(4).Background(t.SurfaceHover).Children(func() {
		formTitle := "Add environment variable"
		if a.podEnvEditing != "" {
			formTitle = "Edit " + a.podEnvEditing
		}
		ui.Text(c, formTitle).FontSize(12).FontWeight(650).Wrap()
		if a.podEnvEditing == "" {
			ui.TextInput(c, &a.podEnvName).Label("Environment variable name").Placeholder("ENV_NAME").FillWidth()
		} else {
			ui.Text(c, a.podEnvEditing).FontSize(12).FontWeight(600)
		}
		ui.TextInput(c, &a.podEnvValue).Label("Environment variable value").Placeholder("Plain value").FillWidth()
		ui.Row(c).Gap(6).Justify(ui.End).Wrap().Children(func() {
			if ui.Button(c, "Cancel").MinHeight(36).Disabled(a.podEnvBusy).Clicked() {
				a.podEnvContainer, a.podEnvEditing, a.podEnvErr = "", "", ""
			}
			label := "Add variable"
			if a.podEnvEditing != "" {
				label = "Save variable"
			}
			name := strings.TrimSpace(a.podEnvName)
			save := ui.Button(c, label).MinHeight(36).Background(t.Accent).TextColor(t.AccentText).
				Disabled(a.podEnvBusy || name == "")
			if save.Clicked() {
				value := a.podEnvValue
				a.changePodEnvironment(target, container, init, name, &value)
			}
		})
	})
}

func (a *App) changePodEnvironment(target cluster.Identity, container string, init bool, name string, value *string) {
	if a.podEnvBusy || a.podEnvSwitching || a.podEnvMutate == nil {
		return
	}
	owner := a.active
	podKey := a.podEnvPod
	contextName := a.current
	a.podEnvGen++
	gen := a.podEnvGen
	a.podEnvBusyGen = gen
	a.podEnvContext = contextName
	a.podEnvBusy = true
	a.podEnvErr = ""
	mutate := a.podEnvMutate
	go func() {
		err := mutate(context.Background(), contextName, target, container, init, name, value)
		a.mu.Lock()
		if a.active == owner {
			a.finishPodEnvironmentChange(gen, podKey, contextName, err)
		} else if b := a.bodies[owner]; b != nil {
			if b.podEnvBusyGen == gen {
				b.podEnvBusy = false
			}
			if b.podEnvGen == gen && b.podEnvPod == podKey && b.podEnvContext == contextName && a.current == contextName {
				if err != nil {
					b.podEnvErr = publicError(err)
				} else {
					b.podEnvContainer, b.podEnvName, b.podEnvValue, b.podEnvEditing, b.podEnvErr = "", "", "", "", ""
				}
			}
		}
		a.mu.Unlock()
		a.invalidate()
	}()
}

func (a *App) finishPodEnvironmentChange(gen uint64, podKey, contextName string, err error) {
	if a.podEnvBusyGen == gen {
		a.podEnvBusy = false
	}
	if a.podEnvGen != gen || a.podEnvPod != podKey || a.podEnvContext != contextName || a.current != contextName {
		return
	}
	if err != nil {
		a.podEnvErr = publicError(err)
		return
	}
	a.podEnvContainer, a.podEnvName, a.podEnvValue, a.podEnvEditing, a.podEnvErr = "", "", "", "", ""
}

func (a *App) resetPodEnvironmentStateLocked() {
	a.podEnvGen++
	a.podEnvPod, a.podEnvContainer, a.podEnvName, a.podEnvValue, a.podEnvEditing, a.podEnvErr, a.podEnvContext = "", "", "", "", "", "", ""
	a.podEnvInit = false
	for _, b := range a.bodies {
		b.podEnvGen++
		b.podEnvPod, b.podEnvContainer, b.podEnvName, b.podEnvValue, b.podEnvEditing, b.podEnvErr, b.podEnvContext = "", "", "", "", "", "", ""
		b.podEnvInit = false
	}
}

func podEnvValue(env corev1.EnvVar) string {
	if env.ValueFrom == nil {
		if env.Value == "" {
			return `""`
		}
		return env.Value
	}
	source := env.ValueFrom
	switch {
	case source.FieldRef != nil:
		return fmt.Sprintf("fieldRef(%s:%s)", source.FieldRef.APIVersion, source.FieldRef.FieldPath)
	case source.ResourceFieldRef != nil:
		ref := source.ResourceFieldRef
		text := ref.Resource
		if ref.ContainerName != "" {
			text = ref.ContainerName + "/" + text
		}
		if !ref.Divisor.IsZero() {
			text += ", divisor=" + ref.Divisor.String()
		}
		return "resourceFieldRef(" + text + ")"
	case source.ConfigMapKeyRef != nil:
		return envKeyRef("configMapKeyRef", source.ConfigMapKeyRef.LocalObjectReference, source.ConfigMapKeyRef.Key, source.ConfigMapKeyRef.Optional)
	case source.SecretKeyRef != nil:
		return envKeyRef("secretKeyRef", source.SecretKeyRef.LocalObjectReference, source.SecretKeyRef.Key, source.SecretKeyRef.Optional)
	default:
		return "valueFrom"
	}
}

func envKeyRef(kind string, ref corev1.LocalObjectReference, key string, optional *bool) string {
	text := ref.Name + "/" + key
	if optional != nil && *optional {
		text += ", optional"
	}
	return kind + "(" + text + ")"
}

func podEnvFromValue(source corev1.EnvFromSource) string {
	prefix := ""
	if source.Prefix != "" {
		prefix = ", prefix=" + source.Prefix
	}
	switch {
	case source.ConfigMapRef != nil:
		return "envFrom configMap(" + source.ConfigMapRef.Name + optionalSuffix(source.ConfigMapRef.Optional) + prefix + ")"
	case source.SecretRef != nil:
		return "envFrom secret(" + source.SecretRef.Name + optionalSuffix(source.SecretRef.Optional) + prefix + ")"
	default:
		return "envFrom"
	}
}

func optionalSuffix(optional *bool) string {
	if optional != nil && *optional {
		return ", optional"
	}
	return ""
}
