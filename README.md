# yubi-tui

TUI for Yubikey authenticator using [Bubbletea](https://github.com/charmbracelet/bubbletea). Works as a wrapper around `ykman` for getting keys.

Requires `YUBIKEY_SERIAL_NUMBER` environment variable to be set, so it knows what device to query for codes.

## Installation

```bash
git clone https://github.com/justinbrumley/yubi-tui.git
cd yubi-tui
go install
```

## Usage

```bash
yubi
```

## Keybinds

| Key(s)         | Description         |
| :----------- | :--------------: |
| `j, k` | Navigate list of applications. |
| `c, Enter`    | Copy current code under cursor to clipboard. If application requires touch, then it will prompt for a touch first. Once the key is touched and code is generated, then it'll be automatically copied to the clipboard.   |
