// Package launcher inicializa um produto NanoSuite com runtime compartilhado.
package launcher

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nanotube/nanotube-web/internal/app"
	"github.com/nanotube/nanotube-web/internal/auth"
	"github.com/nanotube/nanotube-web/internal/desktop"
	"github.com/nanotube/nanotube-web/internal/diagnostics"
	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/product"
	"github.com/nanotube/nanotube-web/internal/server"
	"github.com/nanotube/nanotube-web/internal/services"
	"github.com/nanotube/nanotube-web/internal/storage"
)

// Run inicializa banco, serviços, assets e o runtime desktop/web de um produto.
func Run(spec product.Spec, embeddedAssets fs.FS) error {
	if err := app.ConfigureNamespace(spec.DataNamespace); err != nil {
		return err
	}
	flags := flag.NewFlagSet(spec.ID, flag.ExitOnError)
	diagnosticsMode := flags.Bool("diagnostics", false, "imprime relatório de diagnóstico e sai")
	versionMode := flags.Bool("version", false, "imprime a versão e sai")
	tvMode := flags.Bool("tv", false, "inicia diretamente no Modo TV")
	devtools := flags.Bool("devtools", false, "abre o inspetor DevTools na inicialização")
	serverMode := flags.Bool("server", false, "executa em modo servidor HTTP / Web sem janela nativa Wails")
	port := flags.Int("port", 0, "porta HTTP no modo servidor (0 seleciona uma porta livre)")
	noBrowser := flags.Bool("no-browser", false, "não abrir o navegador automaticamente no modo servidor")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *versionMode {
		fmt.Printf("%s v%s\n", spec.Name, diagnostics.Version)
		return nil
	}
	if *diagnosticsMode {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		diagnostics.Collect(ctx).Print(os.Stdout)
		return nil
	}

	logFile, err := app.OpenLogFile()
	if err != nil {
		return fmt.Errorf("abrir log do %s: %w", spec.Name, err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	dataDir, err := app.DataDirFor(spec.DataNamespace)
	if err != nil {
		return fmt.Errorf("obter diretório de dados do %s: %w", spec.Name, err)
	}
	dbPath := filepath.Join(dataDir, spec.DatabaseName)
	clonedLegacyDatabase, err := prepareProductDatabase(spec, dbPath)
	if err != nil {
		return fmt.Errorf("preparar dados iniciais do %s: %w", spec.Name, err)
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("abrir SQLite do %s: %w", spec.Name, err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		return fmt.Errorf("migrar SQLite do %s: %w", spec.Name, err)
	}
	if err := pruneLegacyData(db, spec); err != nil {
		return fmt.Errorf("isolar dados do %s: %w", spec.Name, err)
	}

	repo := storage.NewRepository(db)
	secretStore := storage.NewKeyringSecretStore(spec.KeyringService)
	if clonedLegacyDatabase {
		migrateLegacySecrets(context.Background(), spec, repo, secretStore)
	}
	appServices := services.NewAppServices(repo, secretStore)
	assetsFS, err := fs.Sub(embeddedAssets, filepath.ToSlash(filepath.Join("frontend", "dist", spec.FrontendTarget)))
	if err != nil {
		return fmt.Errorf("assets de %s indisponíveis: %w", spec.Name, err)
	}

	hasDisplay := os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	if !*serverMode && hasDisplay {
		log.Printf("=== %s v%s iniciado (Wails v3) | Banco: %s ===", spec.Name, diagnostics.Version, dbPath)
		err := desktop.Run(desktop.Options{
			Name:              spec.Name,
			Description:       spec.Description,
			Title:             spec.Name,
			Width:             spec.DefaultWidth,
			Height:            spec.DefaultHeight,
			IsTVMode:          *tvMode,
			OpenDevTools:      *devtools,
			AssetsFS:          assetsFS,
			Services:          appServices,
			AllowedRPCMethods: spec.AllowedRPCMethods,
			QueueEnabled:      spec.QueueEnabled,
		})
		if err == nil {
			return nil
		}
		log.Printf("aviso: janela nativa de %s falhou (%v); iniciando servidor HTTP", spec.Name, err)
	}

	srv := server.NewServer(appServices, server.Config{
		Addr:              fmt.Sprintf("127.0.0.1:%d", *port),
		StaticDir:         resolveStaticDir(spec.FrontendTarget),
		StaticFS:          assetsFS,
		AllowedRPCMethods: spec.AllowedRPCMethods,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverURL, err := srv.Start(ctx)
	if err != nil {
		return fmt.Errorf("iniciar servidor HTTP do %s: %w", spec.Name, err)
	}
	appURL := serverURL
	if *tvMode {
		appURL += "?tv=1"
	}
	log.Printf("=== %s v%s iniciado (Web) | Banco: %s ===", spec.Name, diagnostics.Version, dbPath)
	log.Printf("Interface disponível em: %s", appURL)
	if !*noBrowser {
		go func() {
			time.Sleep(150 * time.Millisecond)
			_ = auth.OpenBrowser(ctx, appURL)
		}()
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	<-signalChannel
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	return srv.Shutdown(shutdownCtx)
}

func pruneLegacyData(db *sql.DB, spec product.Spec) error {
	statements := []string(nil)
	switch spec.ID {
	case "hummiptv", "nanoiptv":
		statements = []string{
			"DELETE FROM credential_audit",
			"DELETE FROM channel_tags", "DELETE FROM playback_queue", "DELETE FROM playback_queue_preferences",
			"DELETE FROM profile_accounts", "DELETE FROM profile_members", "DELETE FROM profiles WHERE id <> 'default'",
			"DELETE FROM playlist_tags", "DELETE FROM playlist_rules", "DELETE FROM playlist_items", "DELETE FROM playlists", "DELETE FROM smart_presets",
			"DELETE FROM video_bookmarks", "DELETE FROM video_notes", "DELETE FROM favorites", "DELETE FROM queue_items",
			"DELETE FROM recommendation_feedback", "DELETE FROM interest_topics", "DELETE FROM playback_progress",
			"DELETE FROM channel_folder_items", "DELETE FROM channel_folders", "DELETE FROM channel_favorites",
			"DELETE FROM channels", "DELETE FROM videos_fts", "DELETE FROM videos",
			"DELETE FROM settings WHERE key NOT IN ('accent_color', 'theme', 'locale', 'close_behavior', 'tray_seek_seconds', 'thumbnail_size', 'thumbnail_quality')",
		}
	case "hummmusic", "nanomusic":
		statements = []string{
			"DELETE FROM iptv_saved_items", "DELETE FROM iptv_playback_progress", "DELETE FROM iptv_guide_programs",
			"DELETE FROM iptv_guide_channels", "DELETE FROM iptv_items", "DELETE FROM iptv_sources",
		}
	default:
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func prepareProductDatabase(spec product.Spec, targetPath string) (bool, error) {
	if _, err := os.Stat(targetPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	var candidateLegacyPaths []string
	switch spec.ID {
	case "hummtube", "nanotube":
		if legacyDir, err := app.DataDirFor("nanotube-web"); err == nil {
			candidateLegacyPaths = append(candidateLegacyPaths, filepath.Join(legacyDir, "nanotube-web.db"))
		}
	case "hummiptv", "nanoiptv":
		if legacyDir, err := app.DataDirFor("nanoiptv"); err == nil {
			candidateLegacyPaths = append(candidateLegacyPaths, filepath.Join(legacyDir, "nanoiptv.db"))
		}
		if legacyDir, err := app.DataDirFor("nanotube-web"); err == nil {
			candidateLegacyPaths = append(candidateLegacyPaths, filepath.Join(legacyDir, "nanotube-web.db"))
		}
	case "hummmusic", "nanomusic":
		if legacyDir, err := app.DataDirFor("nanomusic"); err == nil {
			candidateLegacyPaths = append(candidateLegacyPaths, filepath.Join(legacyDir, "nanomusic.db"))
		}
		if legacyDir, err := app.DataDirFor("nanotube-web"); err == nil {
			candidateLegacyPaths = append(candidateLegacyPaths, filepath.Join(legacyDir, "nanotube-web.db"))
		}
	}

	for _, legacyPath := range candidateLegacyPaths {
		if _, err := os.Stat(legacyPath); err == nil {
			if err := cloneSQLiteDatabase(legacyPath, targetPath); err != nil {
				return false, err
			}
			log.Printf("dados legados copiados para o primeiro uso do %s; o banco original foi preservado", spec.Name)
			return true, nil
		}
	}
	return false, nil
}

func cloneSQLiteDatabase(sourcePath, targetPath string) error {
	sourceDB, err := storage.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceDB.Close()
	if _, err := sourceDB.Exec("VACUUM INTO ?", targetPath); err != nil {
		return fmt.Errorf("snapshot SQLite: %w", err)
	}
	if err := os.Chmod(targetPath, 0o600); err != nil {
		return fmt.Errorf("proteger snapshot SQLite: %w", err)
	}
	return nil
}

func migrateLegacySecrets(ctx context.Context, spec product.Spec, repo *storage.Repository, target domain.SecretStore) {
	legacyKeyrings := []string{"nanotube-web", "nanoiptv", "nanomusic"}
	for _, keyring := range legacyKeyrings {
		source := storage.NewKeyringSecretStore(keyring)
		keys, err := legacySecretKeys(ctx, spec, repo)
		if err != nil {
			continue
		}
		for _, key := range keys {
			value, readErr := source.Get(ctx, key)
			if errors.Is(readErr, domain.ErrSecretNotFound) || readErr != nil {
				continue
			}
			_ = target.Set(ctx, key, value)
		}
	}
}

func legacySecretKeys(ctx context.Context, spec product.Spec, repo *storage.Repository) ([]domain.SecretKey, error) {
	switch spec.ID {
	case "hummiptv", "nanoiptv":
		sources, err := repo.ListIPTVSources(ctx)
		if err != nil {
			return nil, err
		}
		keys := make([]domain.SecretKey, 0, len(sources))
		for _, source := range sources {
			if source.Config.CredentialRef != "" {
				keys = append(keys, domain.SecretKey(source.Config.CredentialRef))
			}
		}
		return keys, nil
	case "hummmusic", "nanomusic":
		profiles, err := repo.Profiles(ctx)
		if err != nil {
			return nil, err
		}
		keys := []domain.SecretKey{"google.refresh_token"}
		for _, profile := range profiles {
			if profile.IsGuest() {
				continue
			}
			keys = append(keys, auth.RefreshTokenKey(profile.ID), domain.SecretKey("lastfm.session/"+profile.ID))
		}
		return keys, nil
	case "hummtube", "nanotube":
		profiles, err := repo.Profiles(ctx)
		if err != nil {
			return nil, err
		}
		keys := []domain.SecretKey{"google.refresh_token"}
		for _, profile := range profiles {
			if profile.IsGuest() {
				continue
			}
			keys = append(keys, auth.RefreshTokenKey(profile.ID))
		}
		return keys, nil
	default:
		return nil, nil
	}
}

func resolveStaticDir(frontendTarget string) string {
	workingDirectory, _ := os.Getwd()
	targets := []string{frontendTarget}
	switch frontendTarget {
	case "hummtube":
		targets = append(targets, "nanotube")
	case "hummiptv":
		targets = append(targets, "nanoiptv")
	case "hummmusic":
		targets = append(targets, "nanomusic")
	}

	for _, target := range targets {
		staticDir := filepath.Join(workingDirectory, "frontend", "dist", target)
		if _, err := os.Stat(staticDir); err == nil {
			return staticDir
		}
	}

	executable, err := os.Executable()
	if err != nil {
		return filepath.Join(workingDirectory, "frontend", "dist", frontendTarget)
	}
	execDir := filepath.Dir(executable)
	for _, target := range targets {
		staticDir := filepath.Join(execDir, "frontend", "dist", target)
		if _, err := os.Stat(staticDir); err == nil {
			return staticDir
		}
	}
	return filepath.Join(execDir, "frontend", "dist", frontendTarget)
}
