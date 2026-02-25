package config

type Config struct {
	Version   int                 `toml:"version"`
	Layout    string              `toml:"layout"`
	Packages  PackageConfig       `toml:"packages"`
	Hooks     HookConfig          `toml:"hooks"`
	Overrides map[string]Override `toml:"overrides"`
	Ignore    IgnoreConfig        `toml:"ignore"`
	Templates map[string]string   `toml:"templates"`
	Backup    BackupConfig        `toml:"backup"`
}

type PackageConfig struct {
	Common    []string `toml:"common"`
	Darwin    []string `toml:"darwin"`
	LinuxAPT  []string `toml:"linux_apt"`
	LinuxBrew []string `toml:"linux_brew"`
	WSLAPT    []string `toml:"wsl_apt"`
	WSLBrew   []string `toml:"wsl_brew"`
	// NerdFont is the name of the Nerd Font to install on Linux/WSL via
	// Homebrew cask, e.g. "JetBrainsMono" or "FiraCode". Defaults to
	// "FiraCode" when empty.
	NerdFont string `toml:"nerd_font"`
}

type HookConfig struct {
	PreInstall   string `toml:"pre_install"`
	PostInstall  string `toml:"post_install"`
	PreLink      string `toml:"pre_link"`
	PostLink     string `toml:"post_link"`
	PreSync      string `toml:"pre_sync"`
	PostSync     string `toml:"post_sync"`
	PrePush      string `toml:"pre_push"`
	PostPush     string `toml:"post_push"`
	PreUpdate    string `toml:"pre_update"`
	PostUpdate   string `toml:"post_update"`
	PreRollback  string `toml:"pre_rollback"`
	PostRollback string `toml:"post_rollback"`
}

type Override struct {
	Target   string   `toml:"target"`
	Platform []string `toml:"platform"`
	Hostname string   `toml:"hostname"`
}

type IgnoreConfig struct {
	Paths []string `toml:"paths"`
}

type BackupConfig struct {
	Enabled  bool `toml:"enabled"`
	MaxAge   int  `toml:"max_age"`
	MaxCount int  `toml:"max_count"`
}
