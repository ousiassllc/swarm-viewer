package compose

import (
	"runtime"

	"github.com/ousiassllc/swarm-viewer/internal/repository/installer"
	"github.com/ousiassllc/swarm-viewer/internal/repository/release"
	"github.com/ousiassllc/swarm-viewer/internal/service"
	"github.com/ousiassllc/swarm-viewer/internal/service/updater"
)

// newUpdater は新しい版の確認と更新の service を作る。GitHub への通信は repository/release の専用クライアントで、
// swarm-console への接続（repository/report）とは別（Issue #109）。
func (r *Runtime) newUpdater(settings *service.SettingsService) *updater.Service {
	c := r.cfg
	var src updater.Source = release.NewClient("swarm-viewer/" + c.Version)
	if c.Releases != nil {
		src = c.Releases
	}
	var inst updater.Installer = installer.New()
	if c.Installer != nil {
		inst = c.Installer
	}
	goos := c.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	return updater.New(updater.Options{
		Source: src, Installer: inst, Settings: settings, Clock: c.Clock, Log: r.log,
		Version: c.Version, GOOS: goos, Quit: c.Quit,
	})
}
