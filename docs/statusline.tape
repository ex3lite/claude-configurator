Output "__OUTPUT_GIF__"

Set Shell "bash"
Set Width 1400
Set Height 260
Set FontSize 18
Set FontFamily "__NERD_FONT__"
Set LineHeight 1.2
Set Padding 18
Set Margin 0
Set BorderRadius 8
Set WindowBar RingsRight
Set CursorBlink false
Set TypingSpeed 0
Set Theme { "name": "Claude", "black": "#171716", "red": "#c2413b", "green": "#678f69", "yellow": "#c68443", "blue": "#8b6fb1", "magenta": "#9b72cf", "cyan": "#d97757", "white": "#e8e5df", "brightBlack": "#7c7774", "brightRed": "#e05b52", "brightGreen": "#7fa581", "brightYellow": "#dda15e", "brightBlue": "#aa8bd1", "brightMagenta": "#b991e2", "brightCyan": "#e38b6d", "brightWhite": "#ffffff", "background": "#1f1f1d", "foreground": "#d7d2ca", "selection": "#4a332b", "cursor": "#d97757" }

Env LC_ALL "__LOCALE__"
Env LANG "__LOCALE__"
Env TZ "Asia/Irkutsk"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"
Env NO_COLOR ""
Env CLAUDE_CONFIG_STATUSLINE_SUBAGENT_MODEL "claude-sonnet-5"
Env CLAUDE_CONFIG_STATUSLINE_ADVISOR_MODEL "opus"

Hide
Type "clear; tput civis; COLUMNS=120 ./scripts/render-statusline-demo.sh; read -r"
Enter
Wait+Screen /__STATUS_WAIT__/
Show
Sleep 500ms
Screenshot "__OUTPUT_DIR__/statusline-limits.png"
Sleep 500ms
Enter
