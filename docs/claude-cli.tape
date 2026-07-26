Output "__OUTPUT_GIF__"

Set Shell "bash"
Set Width 1400
Set Height 760
Set FontSize 18
Set FontFamily "__NERD_FONT__"
Set LineHeight 1.2
Set Padding 18
Set Margin 0
Set BorderRadius 8
Set WindowBar RingsRight
Set CursorBlink false
Set TypingSpeed 0
Set Theme { "name": "Claude", "black": "#171716", "red": "#c2413b", "green": "#678f69", "yellow": "#c68443", "blue": "#8b6fb1", "magenta": "#9b72cf", "cyan": "#d97757", "white": "#e8e5df", "brightBlack": "#7c7774", "brightRed": "#e05b52", "brightGreen": "#7fa581", "brightYellow": "#dda15e", "brightBlue": "#aa8bd1", "brightMagenta": "#9b72cf", "brightCyan": "#e38b6d", "brightWhite": "#ffffff", "background": "#1f1f1d", "foreground": "#d7d2ca", "selection": "#4a332b", "cursor": "#d97757" }

Env TERM "xterm-256color"
Env COLORTERM "truecolor"
Env NO_COLOR ""
Env LC_ALL "__LOCALE__"
Env LANG "__LOCALE__"

Hide
Type "cd __DEMO_DIR__; export PATH=__BINARY_DIR__:$PATH; clear; claude --name demo --no-chrome --setting-sources '' --settings __SETTINGS_FILE__"
Enter
Wait+Screen /Quick safety check/
Enter
Wait+Screen /__LIMITS_WAIT__/
Type "Reply only: OK"
Enter
Wait+Screen /5h/
Sleep 10s
Show
Sleep 1s
Screenshot "__FULL_SCREENSHOT__"
Sleep 1s
Ctrl+C
