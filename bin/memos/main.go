package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/internal/version"
	"github.com/usememos/memos/server"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db"
)

const (
	greetingBanner = `
███╗   ███╗███████╗███╗   ███╗ ██████╗ ███████╗
████╗ ████║██╔════╝████╗ ████║██╔═══██╗██╔════╝
██╔████╔██║█████╗  ██╔████╔██║██║   ██║███████╗
██║╚██╔╝██║██╔══╝  ██║╚██╔╝██║██║   ██║╚════██║
██║ ╚═╝ ██║███████╗██║ ╚═╝ ██║╚██████╔╝███████║
╚═╝     ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝
`
	defaultConfigFileName = "memos.yaml"
)

var (
	cfgFile         string
	activeConfigDir string
	rootCmd         = &cobra.Command{
		Use:   "memos",
		Short: `An open source, lightweight note-taking service. Easily capture and share your great thoughts.`,
		Run: func(_ *cobra.Command, _ []string) {
			instanceProfile := &profile.Profile{
				Mode:        viper.GetString("mode"),
				Addr:        viper.GetString("addr"),
				Port:        viper.GetInt("port"),
				UNIXSock:    viper.GetString("unix-sock"),
				Data:        viper.GetString("data"),
				Driver:      viper.GetString("driver"),
				DSN:         viper.GetString("dsn"),
				InstanceURL: viper.GetString("instance-url"),
				Version:     version.GetCurrentVersion(viper.GetString("mode")),
				TLSCertFile: resolveConfigPath(viper.GetString("https.cert_file")),
				TLSKeyFile:  resolveConfigPath(viper.GetString("https.key_file")),
			}
			if err := instanceProfile.Validate(); err != nil {
				panic(err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			dbDriver, err := db.NewDBDriver(instanceProfile)
			if err != nil {
				cancel()
				slog.Error("failed to create db driver", "error", err)
				return
			}

			storeInstance := store.New(dbDriver, instanceProfile)
			if err := storeInstance.EnsureSchema(ctx); err != nil {
				cancel()
				slog.Error("failed to ensure schema", "error", err)
				return
			}
			if err := storeInstance.Migrate(ctx); err != nil {
				cancel()
				slog.Error("failed to migrate", "error", err)
				return
			}

			s, err := server.NewServer(ctx, instanceProfile, storeInstance)
			if err != nil {
				cancel()
				slog.Error("failed to create server", "error", err)
				return
			}

			c := make(chan os.Signal, 1)
			// Trigger graceful shutdown on SIGINT or SIGTERM.
			// The default signal sent by the `kill` command is SIGTERM,
			// which is taken as the graceful shutdown signal for many systems, eg., Kubernetes, Gunicorn.
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)

			if err := s.Start(ctx); err != nil {
				if err != http.ErrServerClosed {
					slog.Error("failed to start server", "error", err)
					cancel()
				}
			}

			printGreetings(instanceProfile)

			go func() {
				<-c
				s.Shutdown(ctx)
				cancel()
			}()

			// Wait for CTRL-C.
			<-ctx.Done()
		},
	}
)

func init() {
	viper.SetDefault("mode", profile.ModeDev)
	viper.SetDefault("driver", profile.DriverSQLite)
	viper.SetDefault("port", 21853)
	viper.SetDefault("https.cert_file", "")
	viper.SetDefault("https.key_file", "")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "path to the configuration file")
	rootCmd.PersistentFlags().String("https-cert", "", "path to the TLS certificate file")
	rootCmd.PersistentFlags().String("https-key", "", "path to the TLS key file")
	rootCmd.PersistentFlags().String("mode", profile.ModeDev, `mode of server, can be "prod" or "dev" or "demo"`)
	rootCmd.PersistentFlags().String("addr", "", "address of server")
	rootCmd.PersistentFlags().Int("port", 21853, "port of server")
	rootCmd.PersistentFlags().String("unix-sock", "", "path to the unix socket, overrides --addr and --port")
	rootCmd.PersistentFlags().String("data", "", "data directory")
	rootCmd.PersistentFlags().String("driver", profile.DriverSQLite, "database driver")
	rootCmd.PersistentFlags().String("dsn", "", "database source name(aka. DSN)")
	rootCmd.PersistentFlags().String("instance-url", "", "the url of your memos instance")

	if err := viper.BindPFlag("https.cert_file", rootCmd.PersistentFlags().Lookup("https-cert")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("https.key_file", rootCmd.PersistentFlags().Lookup("https-key")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("mode", rootCmd.PersistentFlags().Lookup("mode")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("addr", rootCmd.PersistentFlags().Lookup("addr")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("unix-sock", rootCmd.PersistentFlags().Lookup("unix-sock")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("data", rootCmd.PersistentFlags().Lookup("data")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("driver", rootCmd.PersistentFlags().Lookup("driver")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("dsn", rootCmd.PersistentFlags().Lookup("dsn")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("instance-url", rootCmd.PersistentFlags().Lookup("instance-url")); err != nil {
		panic(err)
	}

	viper.SetEnvPrefix("memos")
	viper.AutomaticEnv()
	if err := viper.BindEnv("instance-url", "MEMOS_INSTANCE_URL"); err != nil {
		panic(err)
	}
	if err := viper.BindEnv("https.cert_file", "MEMOS_HTTPS_CERT_FILE"); err != nil {
		panic(err)
	}
	if err := viper.BindEnv("https.key_file", "MEMOS_HTTPS_KEY_FILE"); err != nil {
		panic(err)
	}

	cobra.OnInitialize(func() {
		if err := initConfig(); err != nil {
			panic(err)
		}
	})
}

func initConfig() error {
	configPath := cfgFile
	if configPath == "" {
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		configPath = filepath.Join(filepath.Dir(execPath), defaultConfigFileName)
	} else {
		configPath = filepath.Clean(configPath)
	}
	if absPath, err := filepath.Abs(configPath); err == nil {
		configPath = absPath
	}
	activeConfigDir = filepath.Dir(configPath)
	if absDir, err := filepath.Abs(activeConfigDir); err == nil {
		activeConfigDir = absDir
	}
	if err := os.MkdirAll(activeConfigDir, 0o755); err != nil {
		return err
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeDefaultConfig(configPath); err != nil {
			return err
		}
		slog.Info("generated default configuration", "path", configPath)
	}
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	defaultDataDir := filepath.Join(activeConfigDir, "data")
	if viper.GetString("data") == "" {
		viper.Set("data", defaultDataDir)
		slog.Info("using default data directory", "path", defaultDataDir)
	}
	return nil
}

const defaultConfigContent = `mode: dev
addr: ""
port: 21853
unix_sock: ""
data: "./data"
driver: sqlite
dsn: ""
instance_url: ""
https:
  cert_file: ""
  key_file: ""
`

func writeDefaultConfig(path string) error {
	return os.WriteFile(path, []byte(defaultConfigContent), 0o644)
}

func resolveConfigPath(value string) string {
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return value
	}
	if activeConfigDir != "" {
		if absPath, err := filepath.Abs(filepath.Join(activeConfigDir, value)); err == nil {
			return absPath
		}
		return filepath.Join(activeConfigDir, value)
	}
	if absPath, err := filepath.Abs(value); err == nil {
		return absPath
	}
	return value
}

func printGreetings(profile *profile.Profile) {
	if profile.IsDev() {
		println("Development mode is enabled")
		println("DSN: ", profile.DSN)
	}
	fmt.Printf(`---
Server profile
version: %s
data: %s
addr: %s
port: %d
unix-sock: %s
mode: %s
driver: %s
---
`, profile.Version, profile.Data, profile.Addr, profile.Port, profile.UNIXSock, profile.Mode, profile.Driver)

	print(greetingBanner)
	if len(profile.UNIXSock) == 0 {
		if len(profile.Addr) == 0 {
			fmt.Printf("Version %s has been started on port %d\n", profile.Version, profile.Port)
		} else {
			fmt.Printf("Version %s has been started on address '%s' and port %d\n", profile.Version, profile.Addr, profile.Port)
		}
	} else {
		fmt.Printf("Version %s has been started on unix socket %s\n", profile.Version, profile.UNIXSock)
	}
	fmt.Printf(`---
See more in:
👉Website: %s
👉GitHub: %s
---
`, "https://usememos.com", "https://github.com/usememos/memos")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
