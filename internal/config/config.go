package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Accepted HERDR_GATEWAY_SELECTION values.
const (
	// GatewaySelectionOrdered registers with the first healthy entry in
	// configured order: an explicit list is a priority, not a preference.
	GatewaySelectionOrdered = "ordered"
	// GatewaySelectionLatency ranks healthy entries by measured round trip. It
	// only fits interchangeable endpoints, such as the community gateway list.
	GatewaySelectionLatency = "latency"
)

type Config struct {
	Host           string
	Port           int
	PluginPort     int
	Token          string
	InstanceID     string
	AllowedOrigins []string
	WebRoot        string
	HerdrBin       string
	// Session is the herdr session the relay mirrors. Herdr keeps every named
	// session on its own socket, so this decides SocketPath; "default" is the
	// session a plain `herdr` command talks to. The relay never creates one.
	Session    string
	SocketPath string
	// QueuePath is the versioned task board (queue/tasks.json) the phone shows
	// next to the live agents. Relative paths resolve against the working
	// directory, which for the repo checkout is where the board lives.
	QueuePath string
	// WatchStatusPath is the watchdog's own state file (manager-watch.status).
	// The phone's watchdog panel reads it; the relay only hands it over.
	WatchStatusPath string
	// OrchestrationPath is the central registry (orchestration.json): which
	// sessions exist, what each is called and where its board lives.
	OrchestrationPath string
	PollInterval      float64
	RuntimeDir        string
	LogFormat         string
	LogLevel          slog.Level
	ReleaseRoot       string
	ServiceName       string

	// GatewayURL is the configured tie-break leader, kept equal to
	// GatewayURLs[0] so readers that only know one gateway keep working. The
	// transport may select another healthy entry at runtime.
	GatewayURL  string
	GatewayURLs []string
	// GatewaySelection is how the transport picks among GatewayURLs: "ordered"
	// registers with the first healthy entry in configured order, "latency"
	// with the lowest-latency healthy one. The loader normalises it, so no
	// reader validates it again.
	GatewaySelection    string
	WebRTCUDPPort       int
	ForceRelayTransport bool
	PortMappingEnabled  bool
	// RearmBootstrap starts every process with an empty device list and a fresh
	// one-use bootstrap invitation. Only the quick-tunnel flow sets it: its app
	// origin changes each launch, so no enrolled credential can be presented again.
	RearmBootstrap bool

	CacheDir   string
	ConfigHome string
	DataHome   string
}

func Load() (*Config, error) {
	cfg := &Config{
		Host:         envOr("HERDR_RELAY_HOST", "127.0.0.1"),
		Port:         envIntOr("HERDR_RELAY_PORT", 8375),
		PluginPort:   envIntOr("HERDR_RELAY_PLUGIN_PORT", 8376),
		Token:        os.Getenv("HERDR_RELAY_TOKEN"),
		InstanceID:   os.Getenv("HERDR_RELAY_INSTANCE_ID"),
		WebRoot:      os.Getenv("HERDR_WEB_ROOT"),
		HerdrBin:     os.Getenv("HERDR_BIN"),
		SocketPath:   os.Getenv("HERDR_SOCKET_PATH"),
		PollInterval: envFloatOr("HERDR_RELAY_POLL_INTERVAL", 2.0),
		LogFormat:    envOr("HERDR_RELAY_LOG_FORMAT", "text"),
		ServiceName:  envOr("HERDR_RELAY_SERVICE_NAME", defaultServiceName()),

		WebRTCUDPPort:       envIntOr("HERDR_WEBRTC_UDP_PORT", 0),
		ForceRelayTransport: envBoolOr("HERDR_TRANSPORT_FORCE_RELAY", false),
		PortMappingEnabled:  envBoolOr("HERDR_REACHABILITY_PORT_MAPPING", true),
		RearmBootstrap:      envBoolOr("HERDR_RELAY_REARM_BOOTSTRAP", false),
	}

	logLevel, err := parseLogLevel(os.Getenv("HERDR_RELAY_LOG_LEVEL"))
	if err != nil {
		return nil, err
	}
	cfg.LogLevel = logLevel

	if origins := os.Getenv("HERDR_ALLOWED_ORIGINS"); origins != "" {
		for _, o := range strings.Split(origins, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	}

	// HERDR_GATEWAY_URL is an ordered candidate list. The relay probes the
	// entries concurrently; HERDR_GATEWAY_SELECTION decides what the order
	// means. A single value is one entry and behaves exactly as it always did.
	cfg.GatewayURLs = parseGatewayURLs(os.Getenv("HERDR_GATEWAY_URL"))
	if len(cfg.GatewayURLs) > 0 {
		cfg.GatewayURL = cfg.GatewayURLs[0]
	}
	cfg.GatewaySelection = parseGatewaySelection(os.Getenv("HERDR_GATEWAY_SELECTION"))

	cfg.ConfigHome = envOr("XDG_CONFIG_HOME", filepath.Join(homeDir(), ".config"))
	cacheHome := envOr("XDG_CACHE_HOME", filepath.Join(homeDir(), ".cache"))
	cfg.DataHome = envOr("XDG_DATA_HOME", filepath.Join(homeDir(), ".local", "share"))
	cfg.ReleaseRoot = os.Getenv("HERDR_RELEASE_ROOT")
	if cfg.ReleaseRoot == "" {
		cfg.ReleaseRoot = installedReleaseRoot()
	}
	if cfg.ReleaseRoot == "" {
		cfg.ReleaseRoot = filepath.Join(cfg.DataHome, "herdr-mobile-relay")
	}

	if cfg.Session == "" {
		cfg.Session = strings.TrimSpace(os.Getenv("HERDR_SESSION"))
	}
	if cfg.Session == "" {
		cfg.Session = DefaultSession
	}
	if cfg.SocketPath == "" {
		cfg.SocketPath = SessionSocketPath(cfg.ConfigHome, cfg.Session)
	}

	if cfg.QueuePath == "" {
		cfg.QueuePath = os.Getenv("HERDR_QUEUE_PATH")
	}
	if cfg.QueuePath == "" {
		cfg.QueuePath = filepath.Join("queue", "tasks.json")
	}

	if cfg.WatchStatusPath == "" {
		cfg.WatchStatusPath = os.Getenv("HERDR_WATCH_STATUS_PATH")
	}
	if cfg.WatchStatusPath == "" {
		cfg.WatchStatusPath = filepath.Join(stateHome(), "manager-watch.status")
	}

	if cfg.OrchestrationPath == "" {
		cfg.OrchestrationPath = os.Getenv("HERDR_ORCHESTRATION_PATH")
	}
	if cfg.OrchestrationPath == "" {
		cfg.OrchestrationPath = filepath.Join(homeDir(), "Documents", "github", "herdr-manager-wake", "orchestration.json")
	}

	cfg.RuntimeDir = resolveRuntimeDir(cfg.ConfigHome)
	cfg.CacheDir = filepath.Join(cacheHome, "herdr-mobile-relay")

	if cfg.WebRoot == "" {
		cfg.WebRoot = defaultWebRoot()
	}

	if cfg.HerdrBin == "" {
		cfg.HerdrBin = findHerdrBin()
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// DefaultSession is the session a bare `herdr` command talks to, and the one the
// relay mirrors unless HERDR_SESSION says otherwise.
const DefaultSession = "default"

// SessionSocketPath is where herdr keeps a session's socket. The default session
// lives next to herdr's config; every named one lives under sessions/<name>.
func SessionSocketPath(configHome, session string) string {
	if session == "" || session == DefaultSession {
		return filepath.Join(configHome, "herdr", "herdr.sock")
	}
	return filepath.Join(configHome, "herdr", "sessions", session, "herdr.sock")
}

// validSessionName keeps a configured name from escaping the sessions directory.
func validSessionName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '-', char == '_', char == '.':
		default:
			return false
		}
	}
	return true
}

// stateHome is where the machine keeps state that outlives a login: the watchdog
// writes its status file here.
func stateHome() string {
	if configured := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); configured != "" {
		return configured
	}
	return filepath.Join(homeDir(), ".local", "state")
}

func (c *Config) validate() error {
	if c.Token == "" && c.Host != "127.0.0.1" && c.Host != "::1" && c.Host != "localhost" {
		return fmt.Errorf("refusing to bind tokenless relay to non-loopback address %s", c.Host)
	}
	if c.Token != "" && len(c.Token) != 32 {
		return errors.New("relay key must be exactly 32 bytes")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d", c.Port)
	}
	if !validSessionName(c.Session) {
		return fmt.Errorf("invalid herdr session name %q: letters, digits, dot, dash and underscore only", c.Session)
	}
	for _, gateway := range c.GatewayURLs {
		parsed, err := url.Parse(gateway)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "ws" && parsed.Scheme != "wss") {
			return fmt.Errorf("invalid gateway url %q: want ws:// or wss:// base url", gateway)
		}
	}
	if len(c.GatewayURLs) > 0 && c.Token == "" {
		return fmt.Errorf("gateway url requires a relay key: the gateway path derives its credentials from it")
	}
	return nil
}

func resolveRuntimeDir(configHome string) string {
	if env := os.Getenv("HERDR_RELAY_ENV"); env != "" {
		return filepath.Dir(env)
	}
	if dir := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(configHome, "herdr-mobile-relay")
}

func defaultWebRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return "web"
	}
	if root := installedReleaseRoot(); root != "" {
		if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
			return filepath.Join(filepath.Dir(resolved), "web")
		}
		return filepath.Join(filepath.Dir(exe), "web")
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "web")
}

func installedReleaseRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	releaseDir := filepath.Dir(resolved)
	releasesDir := filepath.Dir(releaseDir)
	if filepath.Base(releasesDir) != "releases" {
		return ""
	}
	root := filepath.Dir(releasesDir)
	current := filepath.Join(root, "current")
	if _, err := os.Lstat(current); err != nil {
		return ""
	}
	return root
}

func defaultServiceName() string {
	if runtime.GOOS == "darwin" {
		return "com.herdr-mobile-relay.service"
	}
	return "herdr-mobile-relay.service"
}

func findHerdrBin() string {
	candidates := []string{
		filepath.Join(homeDir(), ".local", "bin", "herdr"),
		"/opt/homebrew/bin/herdr",
		"/usr/local/bin/herdr",
		"/home/linuxbrew/.linuxbrew/bin/herdr",
		"/home/linuxbrew/.linuxbrew/opt/herdr/bin/herdr",
	}
	if p, err := lookPath("herdr"); err == nil {
		return p
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return "herdr"
}

func lookPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH", name)
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return h
}

// parseGatewayURLs splits the ordered gateway list. Empty entries are dropped
// so a trailing comma or a stray space in a hand-edited env file configures a
// working relay instead of a phantom gateway.
func parseGatewayURLs(raw string) []string {
	var urls []string
	for _, entry := range strings.Split(raw, ",") {
		if trimmed := strings.TrimRight(strings.TrimSpace(entry), "/"); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}

// parseGatewaySelection normalises the selection rule. Only the community
// gateway list is a set of interchangeable endpoints where latency ranking is
// the point; a hand-listed gateway is a choice the relay must honour, so
// absent, empty and unrecognised values all mean configured order.
func parseGatewaySelection(raw string) string {
	if strings.ToLower(strings.TrimSpace(raw)) == GatewaySelectionLatency {
		return GatewaySelectionLatency
	}
	return GatewaySelectionOrdered
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(raw)); normalized {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid HERDR_RELAY_LOG_LEVEL %q: want debug, info, warn, or error", raw)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBoolOr(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envFloatOr(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
