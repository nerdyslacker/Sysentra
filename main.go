package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"sysentra/internal/accounts"
	appservice "sysentra/internal/app"
	"sysentra/internal/backends"
	"sysentra/internal/drivers"
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
	"sysentra/internal/xbpsconfig"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/sysentra.png
var appIcon []byte

const applicationID = "io.github.nerdyslacker.Sysentra"

func main() {
	inventoryService := appservice.NewInventoryService()
	commandRunner := runner.Exec{}
	packageService := appservice.NewPackageService(backends.NewSystemXBPS(commandRunner), backends.NewFlatpak(commandRunner, backends.WithFlathubMetadata("https://flathub.org/api/v2/appstream", &http.Client{Timeout: 5 * time.Second})))
	disableUnavailableFlatpak(packageService, exec.LookPath)
	repositoryTrustService := appservice.NewRepositoryTrustService(repositories.NewTrustInspector(commandRunner, &http.Client{Timeout: 15 * time.Second}))
	fontService := appservice.NewFontService(fonts.NewInventory(commandRunner))
	transactionClient := transactions.NewSystemDBusClient()
	transactionService := appservice.NewPersistentPackageTransactionService(transactionClient, commandRunner, flatpakplan.New(commandRunner))
	flatpakRemoteService := appservice.NewFlatpakRemoteService(flatpakremote.New(commandRunner))
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	userServiceRoot := filepath.Join(home, "service")
	turnstileRoot := filepath.Join(home, ".config", "service")
	systemService := appservice.NewSystemService(kernel.NewInventory(commandRunner), repositories.NewInventory(commandRunner), mirrors.NewClient(), settings.NewMirrorRecoveryStore(), xbpsconfig.NewInventory(), firmware.NewInventory(commandRunner), firewall.NewInventory(commandRunner), drivers.NewInventory(commandRunner), transactionClient, services.NewUserInventory(commandRunner, userServiceRoot, "/proc"), services.NewUserManager(commandRunner, userServiceRoot), services.NewUserInventory(commandRunner, turnstileRoot, "/proc"), services.NewTurnstileManager(commandRunner, turnstileRoot), shares.NewInventory(), accounts.NewInventory())
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
			application.NewService(repositoryTrustService),
			application.NewService(transactionService),
			application.NewService(flatpakRemoteService),
			application.NewService(systemService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	app.OnShutdown(func() {
		inventoryService.CancelMockScan()
		if err := transactionClient.Close(); err != nil {
			log.Printf("close transaction client: %v", err)
		}
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Sysentra",
		Width:            1180,
		Height:           760,
		MinWidth:         760,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(20, 24, 31),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

type additionalSourceToggler interface {
	SetAdditionalSourceEnabled(string, bool) ([]appservice.AdditionalSourceView, error)
}

func disableUnavailableFlatpak(sources additionalSourceToggler, lookPath func(string) (string, error)) {
	if _, err := lookPath("flatpak"); err != nil {
		_, _ = sources.SetAdditionalSourceEnabled("flatpak", false)
	}
}
