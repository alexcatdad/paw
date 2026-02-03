/**
 * Audit Patterns Configuration
 * Defines common dotfiles patterns, naming conventions, and expected structures
 */

export interface CommonConfig {
  /** Name of the config category */
  name: string;
  /** Description of what this config does */
  description: string;
  /** Common file names for this config */
  fileNames: string[];
  /** Priority: how important is this config (1=essential, 2=recommended, 3=optional) */
  priority: 1 | 2 | 3;
  /** Platform specificity */
  platform?: "darwin" | "linux" | "all";
}

/**
 * Common dotfile configurations that users typically want
 */
export const COMMON_CONFIGS: CommonConfig[] = [
  // Priority 1: Essential configs
  {
    name: "Shell Config",
    description: "Shell configuration (zsh, bash)",
    fileNames: [".zshrc", ".bashrc", ".bash_profile", "zshrc", "bashrc"],
    priority: 1,
    platform: "all",
  },
  {
    name: "Git Config",
    description: "Git configuration and aliases",
    fileNames: [".gitconfig", "gitconfig", ".gitignore_global", "gitignore_global"],
    priority: 1,
    platform: "all",
  },
  {
    name: "SSH Config",
    description: "SSH configuration and keys setup",
    fileNames: [".ssh/config", "ssh/config"],
    priority: 1,
    platform: "all",
  },

  // Priority 2: Recommended configs
  {
    name: "Starship Prompt",
    description: "Cross-shell prompt configuration",
    fileNames: [".config/starship.toml", "starship.toml"],
    priority: 2,
    platform: "all",
  },
  {
    name: "Vim/Neovim",
    description: "Vim or Neovim configuration",
    fileNames: [".vimrc", "vimrc", ".config/nvim/init.vim", ".config/nvim/init.lua", "nvim/init.lua"],
    priority: 2,
    platform: "all",
  },
  {
    name: "Tmux",
    description: "Terminal multiplexer configuration",
    fileNames: [".tmux.conf", "tmux.conf"],
    priority: 2,
    platform: "all",
  },
  {
    name: "Editor Config",
    description: "Universal editor settings",
    fileNames: [".editorconfig", "editorconfig"],
    priority: 2,
    platform: "all",
  },

  // Priority 3: Optional but nice configs
  {
    name: "Alacritty",
    description: "Alacritty terminal configuration",
    fileNames: [".config/alacritty/alacritty.toml", ".config/alacritty/alacritty.yml", "alacritty/alacritty.toml"],
    priority: 3,
    platform: "all",
  },
  {
    name: "Kitty",
    description: "Kitty terminal configuration",
    fileNames: [".config/kitty/kitty.conf", "kitty/kitty.conf"],
    priority: 3,
    platform: "all",
  },
  {
    name: "Wezterm",
    description: "Wezterm terminal configuration",
    fileNames: [".wezterm.lua", "wezterm.lua", ".config/wezterm/wezterm.lua"],
    priority: 3,
    platform: "all",
  },
  {
    name: "Homebrew Bundle",
    description: "Brewfile for package management",
    fileNames: ["Brewfile", ".Brewfile"],
    priority: 3,
    platform: "darwin",
  },
  {
    name: "macOS Defaults",
    description: "macOS system preferences script",
    fileNames: ["macos", ".macos", "macos.sh", "defaults.sh"],
    priority: 3,
    platform: "darwin",
  },
];

/**
 * Expected directory structures for well-organized dotfiles repos
 */
export const RECOMMENDED_STRUCTURES = {
  /** Flat structure: files at root */
  flat: {
    description: "All dotfiles at repository root",
    pattern: /^\.[a-z]/,
  },
  /** Organized: files in categorized directories */
  organized: {
    description: "Dotfiles organized in directories by category",
    expectedDirs: ["shell", "git", "vim", "config", "bin", "scripts"],
  },
  /** XDG-based: following XDG Base Directory spec */
  xdg: {
    description: "XDG Base Directory compliant structure",
    expectedDirs: [".config", ".local"],
  },
};

/**
 * Naming convention patterns
 */
export const NAMING_CONVENTIONS = {
  /** Files that should have dot prefix removed in repo */
  noDotPrefix: {
    description: "Remove leading dot for clarity (e.g., zshrc instead of .zshrc)",
    pattern: /^[a-z][a-z0-9_-]*$/i,
  },
  /** Files that keep dot prefix */
  keepDotPrefix: {
    description: "Keep leading dot (traditional approach)",
    pattern: /^\.[a-z][a-z0-9_-]*$/i,
  },
};
