// Package desktop provê o runtime Wails v3 nativo para o NanoTube Web.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nanotube/nanotube-web/internal/playback"
	"github.com/nanotube/nanotube-web/internal/server"
	"github.com/nanotube/nanotube-web/internal/services"
	"github.com/nanotube/nanotube-web/internal/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
)

// Options controla a inicialização da janela desktop Wails v3.
type Options struct {
	Name              string
	Description       string
	Title             string
	Width             int
	Height            int
	IsTVMode          bool
	OpenDevTools      bool
	AssetsFS          fs.FS
	Services          *services.AppServices
	AllowedRPCMethods map[string][]string
	QueueEnabled      bool
}

// Run inicializa o runtime Wails v3 e executa o loop de eventos da aplicação.
func Run(opts Options) error {
	appServices := opts.Services
	if appServices == nil {
		return errors.New("serviços da aplicação indisponíveis")
	}
	mediaServer, err := startDesktopMediaServer(appServices.MediaProxy)
	if err != nil {
		return fmt.Errorf("iniciar servidor local de mídia: %w", err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = mediaServer.Shutdown(shutdownContext)
	}()

	// O AssetFileServerFS reconhece FRONTEND_DEVSERVER_URL, definido por
	// `wails3 dev`, e encaminha assets/HMR ao Vite. A API permanece no mesmo
	// origin interno do Wails, sem abrir uma porta RPC adicional no desktop.
	rpcHandler := server.NewServer(appServices, server.Config{AllowedRPCMethods: opts.AllowedRPCMethods}).Handler()
	assetHandler := application.AssetFileServerFS(opts.AssetsFS)
	desktopHandler := routeDesktopRequests(rpcHandler, assetHandler)

	app := application.New(application.Options{
		Name:        opts.Name,
		Description: opts.Description,
		Assets: application.AssetOptions{
			Handler: desktopHandler,
		},
	})

	url := "/"

	if opts.IsTVMode {
		url = "/?tv=1"
	}
	allowDevTools := opts.OpenDevTools || os.Getenv("NANOTUBE_DEVTOOLS") == "1"
	keyBindings := map[string]func(window application.Window){}
	if allowDevTools {
		keyBindings["F12"] = func(w application.Window) { w.OpenDevTools() }
		keyBindings["Ctrl+Shift+I"] = func(w application.Window) { w.OpenDevTools() }
	}

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:                  opts.Title,
		Width:                  opts.Width,
		Height:                 opts.Height,
		URL:                    url,
		DevToolsEnabled:        allowDevTools,
		OpenInspectorOnStartup: allowDevTools && opts.OpenDevTools,
		KeyBindings:            keyBindings,
	})

	configureSystemTray(app, window, appServices, opts.Name, opts.QueueEnabled)

	if opts.IsTVMode {
		window.Fullscreen()
	}

	if err := app.Run(); err != nil {
		return fmt.Errorf("wails v3 run: %w", err)
	}

	return nil
}

func startDesktopMediaServer(mediaProxy *playback.MediaProxy) (*http.Server, error) {
	if mediaProxy == nil {
		return nil, errors.New("proxy de mídia indisponível")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	baseURL := "http://" + listener.Addr().String()
	if err := mediaProxy.SetLocalBaseURL(baseURL); err != nil {
		_ = listener.Close()
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/api/media/", allowDesktopMediaCORS(mediaProxy))
	mediaServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		_ = mediaServer.Serve(listener)
	}()
	return mediaServer, nil
}

func allowDesktopMediaCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Access-Control-Allow-Origin", "*")
		response.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		response.Header().Set("Access-Control-Allow-Headers", "Range")
		response.Header().Set("Access-Control-Expose-Headers", "Accept-Ranges, Content-Length, Content-Range, Content-Type")
		response.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(response, request)
	})
}

const (
	closeBehaviorSetting = "close_behavior"
	traySeekSetting      = "tray_seek_seconds"
)

// configureSystemTray mantém a janela e o player acessíveis sem duplicar a
// lógica de reprodução: o tray emite os mesmos comandos consumidos pela UI.
func configureSystemTray(app *application.App, window application.Window, appServices *services.AppServices, productName string, queueEnabled bool) {
	if !systemTrayAvailable() {
		// O suporte SNI/AppIndicator não está presente em todas as sessões
		// Linux (por exemplo, GNOME sem a extensão AppIndicator). Não criar o
		// objeto Wails nesse caso evita o erro DBus e deixa o fechamento normal.
		if app.Logger != nil {
			app.Logger.Info("bandeja do sistema indisponível nesta sessão; seguindo sem tray")
		}
		return
	}
	tray := app.SystemTray.New().
		SetIcon(icons.SystrayLight).
		SetDarkModeIcon(icons.SystrayDark).
		AttachWindow(window)
	tray.SetTooltip(productName)
	var trayMu sync.Mutex
	rebuildMenu := func() {
		trayMu.Lock()
		defer trayMu.Unlock()
		menu := app.NewMenu()
		menu.Add("Mostrar " + productName).OnClick(func(_ *application.Context) { window.Show().Focus() })
		menu.AddSeparator()
		menu.Add("Reproduzir / Pausar").OnClick(func(_ *application.Context) { emitPlayerCommand(window, "play_pause", 0) })
		if queueEnabled {
			menu.Add("Mídia anterior").OnClick(func(_ *application.Context) { emitPlayerCommand(window, "previous", 0) })
			menu.Add("Próxima mídia").OnClick(func(_ *application.Context) { emitPlayerCommand(window, "next", 0) })
		}
		seekSeconds := traySeekSeconds(appServices)
		menu.Add(fmt.Sprintf("Retroceder %d segundos", seekSeconds)).OnClick(func(_ *application.Context) {
			emitPlayerCommand(window, "seek_relative", -seekSeconds)
		})
		menu.Add(fmt.Sprintf("Avançar %d segundos", seekSeconds)).OnClick(func(_ *application.Context) {
			emitPlayerCommand(window, "seek_relative", seekSeconds)
		})
		menu.Add("Aumentar volume").OnClick(func(_ *application.Context) { emitPlayerCommand(window, "volume_up", 0) })
		menu.Add("Diminuir volume").OnClick(func(_ *application.Context) { emitPlayerCommand(window, "volume_down", 0) })
		menu.Add("Ativar / desativar mudo").OnClick(func(_ *application.Context) { emitPlayerCommand(window, "toggle_mute", 0) })
		if queueEnabled {
			menu.Add("Abrir fila").OnClick(func(_ *application.Context) {
				window.Show().Focus()
				emitPlayerCommand(window, "open_queue", 0)
			})
		}
		menu.AddSeparator()
		menu.Add("Sair").OnClick(func(_ *application.Context) { app.Quit() })
		tray.SetMenu(menu)
	}
	rebuildMenu()
	tray.OnClick(tray.ToggleWindow)
	traySettings := newTraySettingsController(newTrayVisibility(tray), rebuildMenu)
	applyTrayEnabled := func(enabled bool) {
		if err := traySettings.SetEnabled(enabled); err != nil && app.Logger != nil {
			app.Logger.Warn("não foi possível atualizar a visibilidade da bandeja", "enabled", enabled, "error", err)
		}
	}
	// Antes de App.Run, o SystemTray ainda não possui implementação nativa e
	// Show/Hide seriam ignorados. Aplicar no evento de startup também garante
	// que o item StatusNotifier já esteja registrado no D-Bus do Linux.
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		applyTrayEnabled(trayEnabled(appServices))
	})
	appServices.SubscribeSettingChanges(func(key, value string) {
		if err := traySettings.ApplySetting(key, value); err != nil && app.Logger != nil {
			app.Logger.Warn("não foi possível aplicar configuração da bandeja", "key", key, "error", err)
		}
	})

	var closeMu sync.Mutex
	isQuitting := false
	isPromptOpen := false
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		closeMu.Lock()
		defer closeMu.Unlock()
		if isQuitting {
			return
		}
		if !trayEnabled(appServices) {
			isQuitting = true
			return
		}
		behavior, exists, err := appServices.Repo.Setting(context.Background(), closeBehaviorSetting)
		if err == nil && exists && behavior == "quit" {
			isQuitting = true
			return
		}
		event.Cancel()
		if err == nil && exists && behavior == "minimize" {
			window.Hide()
			return
		}
		if isPromptOpen {
			return
		}
		isPromptOpen = true
		dialog := app.Dialog.Question().
			SetTitle("Fechar " + productName).
			SetMessage("Deseja minimizar para a bandeja ou encerrar o aplicativo?").
			AttachToWindow(window)
		minimizeButton := dialog.AddButton("Minimizar para a bandeja").OnClick(func() {
			_ = appServices.Repo.SetSetting(context.Background(), closeBehaviorSetting, "minimize")
			closeMu.Lock()
			isPromptOpen = false
			closeMu.Unlock()
			window.Hide()
		})
		dialog.AddButton("Sair").OnClick(func() {
			_ = appServices.Repo.SetSetting(context.Background(), closeBehaviorSetting, "quit")
			closeMu.Lock()
			isPromptOpen = false
			isQuitting = true
			closeMu.Unlock()
			app.Quit()
		})
		cancelButton := dialog.AddButton("Cancelar").OnClick(func() {
			closeMu.Lock()
			isPromptOpen = false
			closeMu.Unlock()
		})
		dialog.SetDefaultButton(minimizeButton)
		dialog.SetCancelButton(cancelButton)
		dialog.Show()
	})
}

func trayEnabled(appServices *services.AppServices) bool {
	value, exists, err := appServices.Repo.Setting(context.Background(), storage.SettingTrayEnabled)
	return err != nil || !exists || value != "0"
}

func traySeekSeconds(appServices *services.AppServices) int {
	value, exists, err := appServices.Repo.Setting(context.Background(), traySeekSetting)
	if err != nil || !exists {
		return 10
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 10
	}
	switch seconds {
	case 5, 10, 15, 30, 60:
		return seconds
	default:
		return 10
	}
}

func emitPlayerCommand(window application.Window, command string, value int) {
	window.ExecJS(fmt.Sprintf(
		"window.dispatchEvent(new CustomEvent('nanotube-player-command',{detail:{command:%q,value:%d}}));",
		command, value,
	))
}

func routeDesktopRequests(rpcHandler, assetHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") {
			rpcHandler.ServeHTTP(response, request)
			return
		}
		assetHandler.ServeHTTP(response, request)
	})
}
