package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/damonto/sigmo/internal/app/forwarder"
	hnetwork "github.com/damonto/sigmo/internal/app/handler/network"
	"github.com/damonto/sigmo/internal/app/httpapi"
	"github.com/damonto/sigmo/internal/app/router"
	pcall "github.com/damonto/sigmo/internal/pkg/call"
	"github.com/damonto/sigmo/internal/pkg/internet"
	"github.com/damonto/sigmo/internal/pkg/modem"
	"github.com/damonto/sigmo/internal/pkg/settings"
	"github.com/damonto/sigmo/internal/pkg/storage"
	"github.com/damonto/sigmo/internal/pkg/validator"
	"github.com/damonto/sigmo/internal/pkg/websheet"
	"github.com/damonto/sigmo/internal/pkg/wificalling"
)

var (
	BuildVersion  string
	listenAddress string
	dbPath        string
	debug         bool
	showVersion   bool
)

func init() {
	flag.StringVar(&listenAddress, "listen-address", "0.0.0.0:9527", "HTTP listen address")
	flag.StringVar(&dbPath, "db-path", "", "path to SQLite database")
	flag.BoolVar(&debug, "debug", false, "enable debug logging and internal error responses")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
}

func main() {
	flag.Parse()
	if showVersion {
		fmt.Println(BuildVersion)
		return
	}
	applyLogLevel(debug)
	httpapi.SetExposeInternalErrors(debug)

	resolvedDBPath, err := resolveDBPath(dbPath)
	if err != nil {
		slog.Error("configure storage", "error", err)
		os.Exit(1)
	}
	db, err := storage.Open(context.Background(), resolvedDBPath)
	if err != nil {
		slog.Error("configure storage", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Warn("close storage", "error", err)
		}
	}()

	store, err := settings.NewStore(context.Background(), db)
	if err != nil {
		slog.Error("load settings", "error", err)
		os.Exit(1)
	}
	slog.Info("server starting", "version", BuildVersion, "listen_address", listenAddress, "db_path", resolvedDBPath)

	registry, err := modem.NewRegistry()
	if err != nil {
		slog.Error("unable to connect modem registry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := registry.Close(); err != nil {
			slog.Warn("close modem registry", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()

	server := echo.New()
	server.Logger = slog.Default()
	server.Validator = validator.New()
	requestLogger := middleware.RequestLogger()
	server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		logged := requestLogger(next)
		return func(c *echo.Context) error {
			if debug {
				return logged(c)
			}
			return next(c)
		}
	})
	server.Use(middleware.RequestID())
	server.Use(middleware.Recover())
	server.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead, http.MethodOptions},
		AllowHeaders: []string{"*"},
	}))
	internetConnector, err := newInternetConnector(store, db)
	if err != nil {
		slog.Error("configure internet connector", "error", err)
		os.Exit(1)
	}
	startupCtx, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	if err := modem.EnableDisabled(startupCtx, registry); err != nil {
		slog.Error("enable disabled modems", "error", err)
	}
	if err := recoverInternetConnections(startupCtx, registry, internetConnector); err != nil {
		slog.Error("recover internet connections", "error", err)
	}
	cancelStartup()
	relay, err := forwarder.New(store, registry, db)
	if err != nil {
		slog.Error("unable to configure message relay", "error", err)
		os.Exit(1)
	}
	websheets := websheet.New(websheet.Config{})
	wifiCalling := wificalling.New(wificalling.Config{
		Store:      db,
		OnIncoming: relay.ForwardWiFiCallingSMS,
		Websheets:  websheets,
	})
	callService := pcall.New(db, wifiCalling)
	networkPreferences, err := modem.NewNetworkPreferences(db)
	if err != nil {
		slog.Error("configure network preferences", "error", err)
		os.Exit(1)
	}
	if err := router.Register(server, router.RegisterConfig{
		Store:              store,
		Registry:           registry,
		Internet:           internetConnector,
		Relay:              relay,
		NetworkPreferences: networkPreferences,
		Storage:            db,
		WiFiCalling:        wifiCalling,
		Calls:              callService,
		Websheets:          websheets,
	}); err != nil {
		slog.Error("configure router", "error", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup

	wg.Go(func() {
		if err := modem.RunEnableDisabled(ctx, registry); err != nil {
			slog.Error("modem enable runner stopped", "error", err)
		}
	})

	wg.Go(func() {
		if err := modem.RunSMSStorageDefaults(ctx, registry, modem.SMSStorageME); err != nil {
			slog.Error("SMS storage defaults stopped", "error", err)
		}
	})

	wg.Go(func() {
		internetConnector.RunAlwaysOn(ctx, registry)
	})

	wg.Go(func() {
		if err := networkPreferences.Run(ctx, registry); err != nil {
			slog.Error("network preferences restore stopped", "error", err)
		}
	})

	wg.Go(func() {
		if err := hnetwork.RunRegistrationRestore(ctx, registry, db); err != nil {
			slog.Error("network registration restore stopped", "error", err)
		}
	})

	wg.Go(func() {
		if err := relay.Run(ctx); err != nil {
			slog.Error("message relay stopped", "error", err)
			stop()
		}
	})

	wg.Go(func() {
		if err := relay.ForwardCalls(ctx, callService); err != nil {
			slog.Error("call notification relay stopped", "error", err)
			stop()
		}
	})

	wg.Go(func() {
		if err := wifiCalling.Run(ctx, registry); err != nil {
			slog.Error("Wi-Fi Calling coordinator stopped", "error", err)
			stop()
		}
	})

	wg.Go(func() {
		if err := callService.Run(ctx); err != nil {
			slog.Error("call service stopped", "error", err)
			stop()
		}
	})

	startConfig := echo.StartConfig{
		Address:         listenAddress,
		HideBanner:      true,
		GracefulTimeout: 5 * time.Second,
	}
	if err := startConfig.Start(ctx, server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http server stopped", "error", err)
		os.Exit(1)
	}
	wg.Wait()
}

func resolveDBPath(path string) (string, error) {
	if path != "" {
		if filepath.IsAbs(path) {
			return path, nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("resolve db path: %w", err)
		}
		return abs, nil
	}
	dataDir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "sigmo", "sigmo.db"), nil
}

func dataDir() (string, error) {
	if value := os.Getenv("XDG_DATA_HOME"); value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("XDG_DATA_HOME %q is relative", value)
		}
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	if home == "" {
		return "", errors.New("user home dir is empty")
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("user home dir %q is relative", home)
	}
	return filepath.Join(home, ".local", "share"), nil
}

func applyLogLevel(debug bool) {
	if debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		return
	}
	slog.SetLogLoggerLevel(slog.LevelInfo)
}

func newInternetConnector(store *settings.Store, db *storage.Store) (*internet.Connector, error) {
	proxyConfig := store.ProxySettings()
	proxy := internet.NewProxy(internet.ProxyConfig{
		ListenAddress: proxyConfig.ListenAddress,
		HTTPPort:      proxyConfig.HTTPPort,
		SOCKS5Port:    proxyConfig.SOCKS5Port,
		Password:      proxyConfig.Password,
	})
	return internet.NewConnector(internet.ConnectorConfig{Proxy: proxy, State: db})
}

func recoverInternetConnections(ctx context.Context, registry *modem.Registry, connector *internet.Connector) error {
	modemMap, err := registry.Modems(ctx)
	if err != nil {
		return fmt.Errorf("list modems: %w", err)
	}
	modems := make([]*modem.Modem, 0, len(modemMap))
	for _, modem := range modemMap {
		modems = append(modems, modem)
	}
	return connector.Recover(ctx, modems)
}
