package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"sysentra/internal/accounts"
	appservice "sysentra/internal/app"
	"sysentra/internal/autostart"
	"sysentra/internal/backends"
	"sysentra/internal/desktopnotify"
	"sysentra/internal/drivers"
	"sysentra/internal/environment"
	"sysentra/internal/firewall"
	"sysentra/internal/firmware"
	"sysentra/internal/flatpakplan"
	"sysentra/internal/flatpakremote"
	"sysentra/internal/fonts"
	"sysentra/internal/kernel"
	"sysentra/internal/mirrors"
	"sysentra/internal/repositories"
	"sysentra/internal/runner"
	"sysentra/internal/services"
	"sysentra/internal/settings"
	"sysentra/internal/shares"
	"sysentra/internal/transactions"
	"sysentra/internal/trayicon"
	"sysentra/internal/xbpsconfig"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/sysentra.png
var appIcon []byte

const applicationID = "io.github.nerdyslacker.Sysentra"

func main() {
	background := backgroundMode(os.Args[1:])
	inventoryService := appservice.NewInventoryService()
	commandRunner := runner.Exec{}
	packageService := appservice.NewPackageService(backends.NewSystemXBPS(commandRunner), backends.NewFlatpak(commandRunner, backends.WithFlathubMetadata("https://flathub.org/api/v2/appstream", &http.Client{Timeout: 5 * time.Second})))
	disableUnavailableFlatpak(packageService, exec.LookPath)
	monitorContext, stopMonitor := context.WithCancel(context.Background())
	var tray *application.SystemTray
	var trayMenu *application.Menu
	var trayStatus *application.MenuItem
	updateMonitor, err := appservice.NewUpdateMonitorService(monitorContext, packageService, settings.NewUpdateChecksStore(), func(result appservice.UpdateCheckResultView, notify bool) {
		label := updateStatusLabel(result.Total)
		icon := appIcon
		if result.Total > 0 {
			if updateIcon, err := trayicon.Updates(appIcon, result.Total); err == nil {
				icon = updateIcon
			} else {
				log.Printf("render update tray icon: %v", err)
			}
		}
		tray.SetIcon(icon)
		trayStatus.SetLabel(label)
		trayMenu.Update()
		tray.SetTooltip(label)
		if notify {
			if err := desktopnotify.Send("Software updates available", label); err != nil {
				log.Printf("update notification: %v", err)
			}
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	repositoryTrustService := appservice.NewRepositoryTrustService(repositories.NewTrustInspector(commandRunner, &http.Client{Timeout: 15 * time.Second}))
	fontService := appservice.NewFontService(fonts.NewInventory(commandRunner))
	transactionClient := transactions.NewSystemDBusClient()
	transactionService := appservice.NewPersistentPackageTransactionService(transactionClient, commandRunner, flatpakplan.New(commandRunner))
	flatpakRemoteService := appservice.NewFlatpakRemoteService(flatpakremote.New(commandRunner))
	autostartManager, err := autostart.New()
	if err != nil {
		log.Fatal(err)
	}
	autostartService := appservice.NewAutostartService(autostartManager)
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	userServiceRoot := filepath.Join(home, "service")
	turnstileRoot := filepath.Join(home, ".config", "service")
	environmentService := appservice.NewEnvironmentService(environment.NewManager(home), transactionClient)
	systemService := appservice.NewSystemService(kernel.NewInventory(commandRunner), repositories.NewInventory(commandRunner), mirrors.NewClient(), settings.NewMirrorRecoveryStore(), xbpsconfig.NewInventory(), firmware.NewInventory(commandRunner), firewall.NewInventory(commandRunner), drivers.NewInventory(commandRunner), transactionClient, services.NewUserInventory(commandRunner, userServiceRoot, "/proc"), services.NewUserManager(commandRunner, userServiceRoot), services.NewUserInventory(commandRunner, turnstileRoot, "/proc"), services.NewTurnstileManager(commandRunner, turnstileRoot), shares.NewInventory(), accounts.NewInventory())
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name:        "Sysentra",
		Description: "Software and system management for Void Linux",
		Icon:        appIcon,
		Linux: application.LinuxOptions{
			ApplicationID: applicationID,
			ProgramName:   applicationID,
		},
		Services: []application.Service{
			application.NewService(inventoryService),
			application.NewService(fontService),
			application.NewService(packageService),
			application.NewService(updateMonitor),
			application.NewService(repositoryTrustService),
			application.NewService(transactionService),
			application.NewService(flatpakRemoteService),
			application.NewService(autostartService),
			application.NewService(systemService),
			application.NewService(environmentService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: applicationID,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window != nil {
					window.Restore()
					window.Show().Focus()
				}
			},
		},
	})
	app.OnShutdown(func() {
		inventoryService.CancelMockScan()
		if err := transactionClient.Close(); err != nil {
			log.Printf("close transaction client: %v", err)
		}
	})

	windowURL := "/"
	if background {
		windowURL = "/?background=1"
	}
	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Sysentra",
		Width:            1180,
		Height:           760,
		MinWidth:         760,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(20, 24, 31),
		Hidden:           background,
		URL:              windowURL,
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		window.Hide()
		event.Cancel()
	})

	tray = app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetLabel("Sysentra")
	tray.SetTooltip("Updates not checked")
	trayMenu = app.NewMenu()
	trayStatus = trayMenu.Add("Updates not checked").SetEnabled(false)
	trayMenu.Add("Check for updates").OnClick(func(*application.Context) {
		go func() {
			if _, err := updateMonitor.CheckForUpdates(); err != nil {
				log.Printf("check for updates: %v", err)
			}
		}()
	})
	trayMenu.Add("Show Sysentra").OnClick(func(*application.Context) { window.Show().Focus() })
	trayMenu.AddSeparator()
	trayMenu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(trayMenu)
	tray.OnClick(func() { window.Show().Focus() })
	app.OnShutdown(stopMonitor)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func updateStatusLabel(count int) string {
	if count == 0 {
		return "System is up to date"
	}
	if count == 1 {
		return "1 update available"
	}
	return fmt.Sprintf("%d updates available", count)
}

func backgroundMode(arguments []string) bool {
	return slices.Contains(arguments, "--background")
}

type additionalSourceToggler interface {
	SetAdditionalSourceEnabled(string, bool) ([]appservice.AdditionalSourceView, error)
}

func disableUnavailableFlatpak(sources additionalSourceToggler, lookPath func(string) (string, error)) {
	if _, err := lookPath("flatpak"); err != nil {
		_, _ = sources.SetAdditionalSourceEnabled("flatpak", false)
	}
}
