package profile

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

// Profile is the configuration to start main server.
type Profile struct {
	// Mode can be "prod" or "dev" or "demo"
	Mode string
	// Addr is the binding address for server
	Addr string
	// Port is the binding port for server
	Port int
	// UNIXSock is the IPC binding path. Overrides Addr and Port
	UNIXSock string
	// Data is the data directory
	Data string
	// DSN points to where memos stores its own data
	DSN string
	// Driver is the database driver
	// sqlite, mysql
	Driver string
	// Version is the current version of server
	Version string
	// InstanceURL is the url of your memos instance.
	InstanceURL string
	// TLSCertFile is the path to HTTPS certificate file
	TLSCertFile string
	// TLSKeyFile is the path to HTTPS key file
	TLSKeyFile string
}

func (p *Profile) IsDev() bool {
	return p.Mode != ModeProd
}

func checkDataDir(dataDir string) (string, error) {
	// Convert to absolute path if relative path is supplied.
	if !filepath.IsAbs(dataDir) {
		relativeDir := filepath.Join(filepath.Dir(os.Args[0]), dataDir)
		absDir, err := filepath.Abs(relativeDir)
		if err != nil {
			return "", err
		}
		dataDir = absDir
	}

	// Trim trailing \ or / in case user supplies
	dataDir = strings.TrimRight(dataDir, "\\/")
	if _, err := os.Stat(dataDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return "", errors.Wrapf(err, "unable to create data folder %s", dataDir)
			}
		} else {
			return "", errors.Wrapf(err, "unable to access data folder %s", dataDir)
		}
	}
	return dataDir, nil
}

func (p *Profile) Validate() error {
	if p.Mode != ModeDemo && p.Mode != ModeDev && p.Mode != ModeProd {
		p.Mode = ModeDemo
	}

	if p.Mode == ModeProd && p.Data == "" {
		if runtime.GOOS == "windows" {
			p.Data = filepath.Join(os.Getenv("ProgramData"), "memos")
			if _, err := os.Stat(p.Data); os.IsNotExist(err) {
				if err := os.MkdirAll(p.Data, 0770); err != nil {
					slog.Error("failed to create data directory", slog.String("data", p.Data), slog.String("error", err.Error()))
					return err
				}
			}
		} else {
			p.Data = "/var/opt/memos"
		}
	}

	dataDir, err := checkDataDir(p.Data)
	if err != nil {
		slog.Error("failed to check dsn", slog.String("data", dataDir), slog.String("error", err.Error()))
		return err
	}

	p.Data = dataDir
	if p.Driver == DriverSQLite && p.DSN == "" {
		dbFile := fmt.Sprintf("memos_%s.db", p.Mode)
		p.DSN = filepath.Join(dataDir, dbFile)
	}

	if err := p.validateTLSFiles(); err != nil {
		return err
	}

	return nil
}

func (p *Profile) validateTLSFiles() error {
	if p.TLSCertFile == "" && p.TLSKeyFile == "" {
		return nil
	}
	if p.TLSCertFile == "" || p.TLSKeyFile == "" {
		return errors.New("both https cert_file and key_file must be provided")
	}
	if _, err := os.Stat(p.TLSCertFile); err != nil {
		return errors.Wrapf(err, "unable to access https cert file: %s", p.TLSCertFile)
	}
	if _, err := os.Stat(p.TLSKeyFile); err != nil {
		return errors.Wrapf(err, "unable to access https key file: %s", p.TLSKeyFile)
	}
	return nil
}
