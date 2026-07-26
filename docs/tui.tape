Output "/tmp/claude-configurator-tui.gif"

Set Shell "bash"
Set Width 1400
Set Height 820
Set FontSize 18
Set FontFamily "__NERD_FONT__"
Set LineHeight 1.15
Set Padding 18
Set Margin 0
Set BorderRadius 8
Set WindowBar RingsRight
Set CursorBlink false
Set TypingSpeed 0
Set Theme { "name": "Claude", "black": "#171716", "red": "#c2413b", "green": "#678f69", "yellow": "#c68443", "blue": "#8b6fb1", "magenta": "#9b72cf", "cyan": "#d97757", "white": "#e8e5df", "brightBlack": "#7c7774", "brightRed": "#e05b52", "brightGreen": "#7fa581", "brightYellow": "#dda15e", "brightBlue": "#aa8bd1", "brightMagenta": "#b991e2", "brightCyan": "#e38b6d", "brightWhite": "#ffffff", "background": "#1f1f1d", "foreground": "#d7d2ca", "selection": "#4a332b", "cursor": "#d97757" }

Env LC_ALL "ru_RU.UTF-8"
Env LANG "ru_RU.UTF-8"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"
Env NO_COLOR ""
Env CLAUDE_CONFIG_NERD_FONT "1"

Hide
Type "rm -rf /tmp/claude-configurator-demo && mkdir -p /tmp/claude-configurator-demo/home/.claude /tmp/claude-configurator-demo/config /tmp/claude-configurator-demo/project && cp docs/demo-settings.json /tmp/claude-configurator-demo/home/.claude/settings.json && clear"
Enter
Wait
Show
Type "HOME=/tmp/claude-configurator-demo/home XDG_CONFIG_HOME=/tmp/claude-configurator-demo/config ./bin/claude-config --no-update --project /tmp/claude-configurator-demo/project"
Enter
Wait+Screen /ГЛАВНОЕ МЕНЮ/
Sleep 500ms
Screenshot docs/tui-main.png
Sleep 500ms
Enter
Wait+Screen /НАСТРОЙКИ/
Sleep 500ms
Screenshot docs/tui-models.png
Sleep 500ms
Ctrl+C
