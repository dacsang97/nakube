package desktop

import "github.com/dacsang97/nakube/internal/cluster"

// YAMLConfirm is the apply dialog. Cancel does not call Apply. Confirm uses
// the identity captured when the dialog opened.
type YAMLConfirm struct {
	Open      bool
	Identity  cluster.Identity
	Body      string
	Live      string
	Predicted string
}

// OpenYAML captures the dry-run preview. Later selection changes do not retarget it.
func OpenYAML(preview *cluster.ApplyPreview, body string) YAMLConfirm {
	if preview == nil {
		return YAMLConfirm{Open: true, Body: body}
	}
	return YAMLConfirm{
		Open:      true,
		Identity:  preview.Identity,
		Body:      body,
		Live:      preview.Live,
		Predicted: preview.Predicted,
	}
}

// ConfirmYAML calls apply only when the dialog is confirmed.
func ConfirmYAML(dialog YAMLConfirm, confirmed bool, apply func(cluster.Identity, string) error) error {
	if !confirmed || !dialog.Open {
		return nil
	}
	return apply(dialog.Identity, dialog.Body)
}
