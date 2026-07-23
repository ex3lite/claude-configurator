# Claude Configurator

[English](README.md) · [Русский](README.ru.md) · [简体中文](README.zh-CN.md)

[![CI](https://github.com/ex3lite/claude-configurator/actions/workflows/ci.yml/badge.svg)](https://github.com/ex3lite/claude-configurator/actions/workflows/ci.yml)
[![Релиз](https://img.shields.io/github/v/release/ex3lite/claude-configurator)](https://github.com/ex3lite/claude-configurator/releases)
[![Лицензия: MIT](https://img.shields.io/badge/license-MIT-22c55e.svg)](LICENSE)

Быстрый терминальный интерфейс для глобальной и проектной настройки Claude
Code. Работает локально, не использует промпты и никуда не отправляет
конфигурацию.

![Интерфейс Claude Configurator](docs/screenshot.svg)

## Возможности

- Глобальные, общие проектные и локальные проектные настройки.
- Выбор основной модели, модели сабагентов, advisor и fallback-цепочки.
- Reasoning, агенты, permissions, sandbox, интерфейс и поведение.
- Поиск, отображение наследования и источника, staged-изменения и diff.
- Защита от конфликтов, автоматические бэкапы и отказ перезаписывать невалидный
  JSON.
- Корректная работа с Git-репозиториями и worktree.
- Один нативный бинарник для macOS, Linux и Windows.

## Установка

### macOS или Linux

```sh
curl -fsSL https://raw.githubusercontent.com/ex3lite/claude-configurator/main/scripts/install.sh | sh
```

Установщик проверяет checksum релиза и устанавливает команды `claude-config`,
`claude-configurator` и `ccfg` в `~/.local/bin`.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/ex3lite/claude-configurator/main/scripts/install.ps1 | iex
```

### Через Go

```sh
go install github.com/ex3lite/claude-configurator/cmd/claude-config@latest
```

Готовые архивы и checksums находятся на
[странице релизов](https://github.com/ex3lite/claude-configurator/releases).

## Использование

```text
claude-config
claude-config --scope global|project|local
claude-config --project /путь/к/проекту
claude-config --help
claude-config --version
```

### Уровни настроек

| Уровень | Файл | Назначение |
|---|---|---|
| Global | `~/.claude/settings.json` | Ваши настройки для всех проектов |
| Project | `.claude/settings.json` | Общие настройки репозитория |
| Local | `.claude/settings.local.json` | Личные переопределения проекта |

Приоритет Claude Code: managed → аргументы CLI → local → project → global.
Claude Configurator редактирует последние три уровня и не меняет
организационные политики.

### Fable как основная модель, Sonnet для сабагентов

Переключитесь на global клавишей `g` и задайте:

```json
{
  "model": "claude-fable-5[1m]",
  "env": {
    "CLAUDE_CODE_SUBAGENT_MODEL": "claude-sonnet-5"
  }
}
```

Для настройки только текущего проекта используйте `p`. Модель сабагентов
применяется ко всем сабагентам, agent teams и workflow-агентам и имеет
приоритет над выбором модели внутри отдельного агента. После сохранения
перезапустите уже работающие сессии Claude Code.

### Клавиши

| Клавиша | Действие |
|---|---|
| `↑/↓`, `j/k` | Навигация |
| `←/→` | Смена категории |
| `g`, `p`, `l` | Global, project, local |
| `Enter` | Редактировать |
| `Space` | Переключить boolean |
| `/` | Поиск |
| `u` | Удалить значение и наследовать |
| `s` | Посмотреть diff и сохранить |
| `r` | Перечитать файлы |
| `?` | Справка |
| `q` | Выход |

## Безопасность и приватность

- Перед записью файл проверяется повторно. Внешнее изменение блокирует
  сохранение, а не перезаписывается.
- Невалидный JSON не заменяется; ошибка содержит файл, строку и столбец.
- Неизвестные приложению настройки сохраняются.
- Бэкапы хранятся в пользовательском cache-каталоге ОС в
  `claude-configurator/backups`; остаются последние 10 копий каждого файла.
- Новые global и local файлы доступны только владельцу.
- Опасные значения вроде `bypassPermissions` требуют второго подтверждения.
- Нет телеметрии, аналитики, доступа к аккаунту и сетевых запросов во время
  работы.

## Решение проблем

- Настройка не применяется: перезапустите Claude Code и проверьте `/status`;
  managed-политика или аргумент CLI могут иметь больший приоритет.
- Ошибка JSON: исправьте указанную `claude-config` позицию и запустите
  `claude doctor`.
- Сохранение заблокировано: файл изменил другой процесс. Нажмите `r`, проверьте
  новое значение и повторите изменение.
- Не нужны цвета: запустите `NO_COLOR=1 claude-config`.

## Разработка

Требуется Go 1.25+.

```sh
go test -race ./...
go vet ./...
go run ./cmd/claude-config
```

Claude Configurator — независимый проект сообщества, не связанный с Anthropic
и не одобренный компанией. Claude является товарным знаком Anthropic.
