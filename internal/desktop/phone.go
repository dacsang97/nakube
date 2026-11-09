package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/dacsang97/nakube/internal/remote"
	"github.com/egoist/mygo/ui"
	qrcode "github.com/skip2/go-qrcode"
)

// Phone adapts the desktop session for the Tailscale listener.
type Phone struct {
	App *App
}

func (p Phone) ArgoApplications(ctx context.Context) ([]remote.ArgoApp, error) {
	apps, err := p.App.session.ListArgoApplications(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]remote.ArgoApp, 0, len(apps))
	for _, app := range apps {
		out = append(out, remote.ArgoApp{
			Name:      app.Name,
			Namespace: app.Namespace,
			Sync:      app.Sync,
			Health:    app.Health,
		})
	}
	return out, nil
}

// StartPhoneListener binds the Tailscale address and publishes a scannable
// endpoint to Settings. A missing CLI or a down tailnet leaves the desktop
// running and makes the setup problem visible there.
func (a *App) StartPhoneListener(ctx context.Context) {
	ip, err := tailscaleIPv4(ctx)
	if err != nil {
		a.setMobileConnection("", nil, errors.New("tailscale is unavailable; sign in to Tailscale and restart NaKube"))
		return
	}
	dnsName, err := tailscaleDNSName(ctx)
	if err != nil {
		a.setMobileConnection("", nil, errors.New("tailscale MagicDNS is unavailable; enable MagicDNS and restart NaKube"))
		return
	}
	qr, err := mobilePairingBitmap(dnsName)
	if err != nil {
		a.setMobileConnection("", nil, errors.New("could not create the mobile pairing code"))
		return
	}
	_, err = remote.ListenTailscale(ctx, func(context.Context) (string, error) {
		return ip, nil
	}, remote.Handler(Phone{App: a}))
	if err != nil {
		a.setMobileConnection("", nil, errors.New("could not listen on the Tailscale address"))
		return
	}
	a.setMobileConnection(fmt.Sprintf("http://%s:%d", dnsName, remote.Port), qr, nil)
}

func (a *App) setMobileConnection(address string, qr *ui.Bitmap, err error) {
	a.mu.Lock()
	a.mobileAddress = address
	a.mobileQR = qr
	a.mobileErr = ""
	if err != nil {
		a.mobileErr = err.Error()
	}
	a.mu.Unlock()
	a.invalidate()
}

func mobilePairingURL(host string) string {
	query := url.Values{}
	query.Set("host", strings.TrimSpace(host))
	query.Set("port", fmt.Sprint(remote.Port))
	return "nakube://connect?" + query.Encode()
}

func mobilePairingBitmap(host string) (*ui.Bitmap, error) {
	code, err := qrcode.New(mobilePairingURL(host), qrcode.Medium)
	if err != nil {
		return nil, err
	}
	bitmap := ui.NewBitmap(code.Image(352))
	bitmap.Pixelated()
	return bitmap, nil
}

func tailscaleDNSName(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "status", "--json").Output()
	if err != nil {
		return "", err
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return "", err
	}
	name := strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName), ".")
	if name == "" {
		return "", errors.New("tailscale status has no MagicDNS name")
	}
	return name, nil
}

func tailscaleIPv4(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "ip", "-4").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
