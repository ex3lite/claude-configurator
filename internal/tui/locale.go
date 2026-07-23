package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type uiLanguage string

const (
	languageAuto  uiLanguage = "auto"
	languageEN    uiLanguage = "en"
	languageRU    uiLanguage = "ru"
	languageZH    uiLanguage = "zh-CN"
	inheritChoice            = "\x00inherit"
	customChoice             = "\x00custom"
)

type preferences struct {
	Language uiLanguage `json:"language"`
}

func loadLanguagePreference() (uiLanguage, string) {
	root, err := os.UserConfigDir()
	if err != nil {
		return languageAuto, ""
	}
	path := filepath.Join(root, "claude-configurator", "config.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return languageAuto, path
	}
	var pref preferences
	if json.Unmarshal(raw, &pref) != nil || !validLanguage(pref.Language) {
		return languageAuto, path
	}
	return pref.Language, path
}

func saveLanguagePreference(path string, language uiLanguage) error {
	if path == "" {
		return fmt.Errorf("cannot resolve the user configuration directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(preferences{Language: language}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func validLanguage(language uiLanguage) bool {
	return language == languageAuto || language == languageEN || language == languageRU || language == languageZH
}

func resolveLanguage(language uiLanguage) uiLanguage {
	if language != languageAuto {
		return language
	}
	for _, name := range []string{
		os.Getenv("LC_ALL"),
		os.Getenv("LC_MESSAGES"),
		os.Getenv("LANGUAGE"),
		os.Getenv("LANG"),
		platformLocale(),
	} {
		normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		switch {
		case strings.HasPrefix(normalized, "ru"):
			return languageRU
		case strings.HasPrefix(normalized, "zh"):
			return languageZH
		case normalized != "" && normalized != "c" && normalized != "posix":
			return languageEN
		}
	}
	return languageEN
}

func (m *Model) tr(key string, args ...any) string {
	text, ok := translations[m.language][key]
	if !ok {
		text = translations[languageEN][key]
	}
	if text == "" {
		text = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

func (m *Model) specLabel(id, fallback string) string {
	key := "spec." + id + ".label"
	if text := translations[m.language][key]; text != "" {
		return text
	}
	return fallback
}

func (m *Model) specDescription(id, fallback string) string {
	key := "spec." + id + ".description"
	if text := translations[m.language][key]; text != "" {
		return text
	}
	return fallback
}

func (m *Model) specPurpose(id, fallback string) string {
	key := "spec." + id + ".purpose"
	if text := translations[m.language][key]; text != "" {
		return text
	}
	return fallback
}

func (m *Model) categoryLabel(category string) string {
	if text := translations[m.language]["category."+category]; text != "" {
		return text
	}
	return category
}

func (m *Model) categoryDescription(category string) string {
	return m.tr("category." + category + ".description")
}

func (m *Model) scopeLabel(scope string) string {
	return m.tr("scope." + scope)
}

func (m *Model) optionLabel(specID, option string) string {
	if option == inheritChoice {
		return m.tr("option.inherit")
	}
	if option == customChoice {
		return m.tr("option.custom")
	}
	if specID == "theme" && strings.HasPrefix(option, "custom:") {
		return m.tr("option.theme.custom", strings.TrimPrefix(option, "custom:"))
	}
	if text := translations[m.language]["option."+specID+"."+option]; text != "" {
		return text
	}
	if text := translations[m.language]["option."+option]; text != "" {
		return text
	}
	return option
}

func (m *Model) helpText() string {
	switch m.language {
	case languageRU:
		return `Навигация
  ↑/↓ или j/k    выбор пункта
  Enter          открыть раздел
  Esc/←          вернуться в меню
  g / p / l      global / project / local

Редактирование
  Enter          изменить значение
  Space          переключить флаг
  u              удалить и наследовать
  /              поиск
  s              проверить и сохранить
  r              перечитать с диска
  q              выйти

Значок ↳ означает наследование, ● — несохранённое изменение.`
	case languageZH:
		return `导航
  ↑/↓ 或 j/k     选择项目
  Enter          打开分类
  Esc/←          返回主菜单
  g / p / l      全局 / 项目 / 本地

编辑
  Enter          修改值
  Space          切换布尔值
  u              清除并继承
  /              搜索
  s              检查并保存
  r              从磁盘重新加载
  q              退出

↳ 表示继承值，● 表示尚未保存的更改。`
	default:
		return `Navigation
  ↑/↓ or j/k    select item
  Enter         open category
  Esc/←         return to main menu
  g / p / l     global / project / local

Editing
  Enter         edit value
  Space         toggle boolean
  u             unset and inherit
  /             search
  s             review and save
  r             reload from disk
  q             quit

Values marked ↳ are inherited. ● marks staged changes.`
	}
}

var translations = map[uiLanguage]map[string]string{
	languageEN: {
		"scope.global":  "Global",
		"scope.project": "Project",
		"scope.local":   "Local",

		"category.Models.description":      "Choose the models used for primary work, subagents, advisor, and fallback.",
		"category.Reasoning.description":   "Control reasoning depth, thinking, and automatic compaction.",
		"category.Agents.description":      "Configure agent teams and how background agents are displayed.",
		"category.Permissions.description": "Define tool permissions and rules across configuration scopes.",
		"category.Safety.description":      "Manage sandbox isolation and file recovery safeguards.",
		"category.Interface.description":   "Adjust Claude Code and Claude Configurator presentation.",
		"category.Behavior.description":    "Tune memory, Git instructions, and the update channel.",

		"option.inherit":                           "Default / inherit",
		"option.custom":                            "Custom model ID…",
		"option.default":                           "Default for account",
		"option.best":                              "Best available",
		"option.fable":                             "Fable 5 · recommended alias · 1M",
		"option.sonnet":                            "Sonnet · latest",
		"option.opus":                              "Opus · latest",
		"option.haiku":                             "Haiku · latest",
		"option.sonnet[1m]":                        "Sonnet · 1M context",
		"option.opus[1m]":                          "Opus · 1M context",
		"option.opusplan":                          "Opus Plan → Sonnet Build",
		"option.claude-fable-5[1m]":                "Fable 5 · fixed ID · 1M",
		"option.claude-sonnet-5":                   "Sonnet 5 · fixed ID",
		"option.theme.auto":                        "Auto · follow terminal",
		"option.theme.dark":                        "Dark",
		"option.theme.light":                       "Light",
		"option.theme.dark-daltonized":             "Dark · color-blind friendly",
		"option.theme.light-daltonized":            "Light · color-blind friendly",
		"option.theme.dark-ansi":                   "Dark · terminal ANSI colors",
		"option.theme.light-ansi":                  "Light · terminal ANSI colors",
		"option.theme.custom":                      "Custom theme · %s",
		"option.ui-language.auto":                  "Auto · system language",
		"option.ui-language.en":                    "English",
		"option.ui-language.ru":                    "Русский",
		"option.ui-language.zh-CN":                 "简体中文",
		"app.source":                               "application",
		"app.language.resolved":                    "Auto resolves to %s",
		"app.language.saved":                       "Interface language updated",
		"app.language.save_failed":                 "Cannot save interface language: %s",
		"header.staged":                            "● staged",
		"header.subtitle":                          "Claude Code settings without hand-editing JSON",
		"action.open":                              "Open",
		"action.edit":                              "Change",
		"action.inherit":                           "Reset to inherit",
		"action.save":                              "Save",
		"action.save_count":                        "Save · %d",
		"action.search":                            "Search",
		"action.scope":                             "Scope",
		"action.help":                              "Keys",
		"action.quit":                              "Quit",
		"action.back":                              "Back",
		"footer.hint.categories":                   "↑↓ choose a section · settings stay staged until you save",
		"footer.hint.settings":                     "↑↓ choose · Space toggles · U removes this scope's value · ↳ means inherited",
		"panel.categories":                         "MAIN MENU",
		"panel.settings":                           "SETTINGS",
		"panel.detail":                             "DETAIL",
		"breadcrumb.menu":                          "Main menu",
		"breadcrumb.search":                        "Search: %s",
		"category.settings_count":                  "%d settings",
		"empty.no_matches":                         "No matching settings",
		"value.inherit":                            "Inherit",
		"value.claude_default":                     "Claude default",
		"value.items":                              "%d items",
		"value.on":                                 "On",
		"value.off":                                "Off",
		"detail.this_scope":                        "THIS SCOPE",
		"detail.effective":                         "EFFECTIVE",
		"detail.what":                              "WHAT IT CONTROLS",
		"detail.why":                               "WHY YOU MAY NEED IT",
		"detail.key":                               "JSON key · %s",
		"detail.inherit_hint":                      "No value is stored here. The effective value is inherited; Enter creates an override.",
		"detail.source":                            "source: %s",
		"detail.suggestions":                       "Choices: %s",
		"detail.permissions_merge":                 "Permission arrays merge across scopes.",
		"detail.app_storage":                       "Stored for Claude Configurator only:\n%s",
		"search.title":                             "Search settings",
		"search.description":                       "Match labels, JSON paths, and descriptions.",
		"editor.custom_model":                      "Custom model ID",
		"editor.edit":                              "Edit %s",
		"editor.apply":                             "Enter apply · Esc cancel · ←→ move",
		"choice.help":                              "↑↓ choose · Enter apply · Esc cancel",
		"choice.inherit_help":                      "“Default / inherit” removes the key from the %s scope instead of writing a fake default value.",
		"list.empty":                               "No values in this scope",
		"list.help":                                "a add · Enter/e edit · d delete · Esc done",
		"list.maximum":                             "Maximum: %d items",
		"diff.more":                                "… %d more changes",
		"diff.help":                                "Enter/y/s save · Esc/n cancel",
		"diff.title":                               "Save %s settings?",
		"help.title":                               "Keyboard help",
		"help.close":                               "Esc or ? close",
		"confirm.danger.title":                     "Dangerous setting",
		"confirm.danger.help":                      "Enter/y apply · Esc/n cancel",
		"confirm.quit.title":                       "Discard staged changes?",
		"confirm.quit.text":                        "Unsaved changes exist in one or more scopes.",
		"confirm.quit.help":                        "Enter/y discard · Esc/n stay",
		"confirm.reload.title":                     "Reload from disk?",
		"confirm.reload.text":                      "All staged changes will be discarded.",
		"confirm.reload.help":                      "Enter/y reload · Esc/n stay",
		"danger.permission-mode.bypassPermissions": "This skips permission prompts and can allow destructive actions.",
		"danger.sandbox.false":                     "Disabling the sandbox removes an important command-isolation layer.",
		"danger.sandbox-unsandboxed.true":          "Unsandboxed commands can access resources outside the isolation boundary.",
		"tiny.text":                                "Terminal is too small.\nResize to at least 48×16.",
		"tiny.quit":                                "q quit",
		"status.cancelled":                         "Change cancelled",
		"status.scope":                             "Editing %s settings",
		"status.inherit":                           "%s now inherits from a lower scope",
		"status.nothing_to_save":                   "Nothing to save in %s",
		"status.max_items":                         "%s accepts at most %d items",
		"status.item_removed":                      "Item removed",
		"status.empty":                             "Value cannot be empty; use u to inherit instead",
		"status.duplicate":                         "Duplicate items are ignored",
		"status.list_updated":                      "List updated",
		"status.staged":                            "%s staged",
		"status.conflict":                          "Save blocked: the file changed on disk. Reload before editing again.",
		"status.save_failed":                       "Save failed: %s",
		"status.saved":                             "Saved. Restart running Claude Code sessions to apply startup settings.",
		"status.reload_failed":                     "Reload failed: %s",
		"status.reloaded":                          "Settings reloaded from disk",
	},
	languageRU: {
		"scope.global":  "Глобально",
		"scope.project": "Проект",
		"scope.local":   "Локально",

		"category.Models":                  "Модели",
		"category.Reasoning":               "Рассуждение",
		"category.Agents":                  "Агенты",
		"category.Permissions":             "Разрешения",
		"category.Safety":                  "Безопасность",
		"category.Interface":               "Интерфейс",
		"category.Behavior":                "Поведение",
		"category.Models.description":      "Основная модель, сабагенты, advisor и резервная цепочка.",
		"category.Reasoning.description":   "Глубина рассуждений, thinking и автоматическое сжатие.",
		"category.Agents.description":      "Команды агентов и отображение фоновых агентов.",
		"category.Permissions.description": "Разрешения инструментов на разных уровнях конфигурации.",
		"category.Safety.description":      "Изоляция sandbox и восстановление изменённых файлов.",
		"category.Interface.description":   "Внешний вид Claude Code и Claude Configurator.",
		"category.Behavior.description":    "Память, Git-инструкции и канал обновлений.",

		"option.inherit":                           "По умолчанию / наследовать",
		"option.custom":                            "Другая модель…",
		"option.default":                           "По умолчанию для аккаунта",
		"option.best":                              "Лучшая доступная",
		"option.fable":                             "Fable 5 · рекомендуемый алиас · 1M",
		"option.sonnet":                            "Sonnet · последняя",
		"option.opus":                              "Opus · последняя",
		"option.haiku":                             "Haiku · последняя",
		"option.sonnet[1m]":                        "Sonnet · контекст 1M",
		"option.opus[1m]":                          "Opus · контекст 1M",
		"option.opusplan":                          "Opus планирует → Sonnet пишет",
		"option.claude-fable-5[1m]":                "Fable 5 · фиксированный ID · 1M",
		"option.claude-sonnet-5":                   "Sonnet 5 · фиксированный ID",
		"option.theme.auto":                        "Авто · как в терминале",
		"option.theme.dark":                        "Тёмная",
		"option.theme.light":                       "Светлая",
		"option.theme.dark-daltonized":             "Тёмная · для дальтонизма",
		"option.theme.light-daltonized":            "Светлая · для дальтонизма",
		"option.theme.dark-ansi":                   "Тёмная · ANSI-цвета терминала",
		"option.theme.light-ansi":                  "Светлая · ANSI-цвета терминала",
		"option.theme.custom":                      "Своя тема · %s",
		"option.ui-language.auto":                  "Авто · язык системы",
		"option.ui-language.en":                    "English",
		"option.ui-language.ru":                    "Русский",
		"option.ui-language.zh-CN":                 "简体中文",
		"app.source":                               "приложение",
		"app.language.resolved":                    "Авто определил: %s",
		"app.language.saved":                       "Язык интерфейса изменён",
		"app.language.save_failed":                 "Не удалось сохранить язык интерфейса: %s",
		"header.staged":                            "● изменено",
		"header.subtitle":                          "Настройки Claude Code без ручного редактирования JSON",
		"action.open":                              "Открыть",
		"action.edit":                              "Изменить",
		"action.inherit":                           "Сбросить → наследовать",
		"action.save":                              "Сохранить",
		"action.save_count":                        "Сохранить · %d",
		"action.search":                            "Поиск",
		"action.scope":                             "Уровень",
		"action.help":                              "Клавиши",
		"action.quit":                              "Выход",
		"action.back":                              "Назад",
		"footer.hint.categories":                   "↑↓ выбрать раздел · изменения применятся только после сохранения",
		"footer.hint.settings":                     "↑↓ выбрать · Space переключить · U удаляет значение этого уровня · ↳ означает наследование",
		"panel.categories":                         "ГЛАВНОЕ МЕНЮ",
		"panel.settings":                           "НАСТРОЙКИ",
		"panel.detail":                             "ПОДРОБНОСТИ",
		"breadcrumb.menu":                          "Главное меню",
		"breadcrumb.search":                        "Поиск: %s",
		"category.settings_count":                  "Настроек: %d",
		"empty.no_matches":                         "Ничего не найдено",
		"value.inherit":                            "Наследуется",
		"value.claude_default":                     "По умолчанию Claude",
		"value.items":                              "Элементов: %d",
		"value.on":                                 "Включено",
		"value.off":                                "Выключено",
		"detail.this_scope":                        "НА ЭТОМ УРОВНЕ",
		"detail.effective":                         "ИТОГОВОЕ ЗНАЧЕНИЕ",
		"detail.what":                              "ЧТО ЭТО МЕНЯЕТ",
		"detail.why":                               "ЗАЧЕМ ЭТО НУЖНО",
		"detail.key":                               "Ключ JSON · %s",
		"detail.inherit_hint":                      "На этом уровне значение не записано. Используется наследуемое; Enter создаст переопределение.",
		"detail.source":                            "источник: %s",
		"detail.suggestions":                       "Варианты: %s",
		"detail.permissions_merge":                 "Списки разрешений объединяются между уровнями.",
		"detail.app_storage":                       "Хранится только для Claude Configurator:\n%s",
		"search.title":                             "Поиск настроек",
		"search.description":                       "Поиск по названию, JSON-пути и описанию.",
		"editor.custom_model":                      "ID другой модели",
		"editor.edit":                              "Изменить: %s",
		"editor.apply":                             "Enter применить · Esc отменить · ←→ курсор",
		"choice.help":                              "↑↓ выбрать · Enter применить · Esc отменить",
		"choice.inherit_help":                      "«По умолчанию / наследовать» удаляет ключ с уровня «%s», а не записывает фиктивное значение.",
		"list.empty":                               "На этом уровне значений нет",
		"list.help":                                "a добавить · Enter/e изменить · d удалить · Esc готово",
		"list.maximum":                             "Максимум: %d",
		"diff.more":                                "… ещё изменений: %d",
		"diff.help":                                "Enter/y/s сохранить · Esc/n отменить",
		"diff.title":                               "Сохранить настройки «%s»?",
		"help.title":                               "Горячие клавиши",
		"help.close":                               "Esc или ? — закрыть",
		"confirm.danger.title":                     "Опасная настройка",
		"confirm.danger.help":                      "Enter/y применить · Esc/n отменить",
		"confirm.quit.title":                       "Отменить изменения?",
		"confirm.quit.text":                        "Есть несохранённые изменения на одном или нескольких уровнях.",
		"confirm.quit.help":                        "Enter/y отменить · Esc/n остаться",
		"confirm.reload.title":                     "Перечитать с диска?",
		"confirm.reload.text":                      "Все несохранённые изменения будут отменены.",
		"confirm.reload.help":                      "Enter/y перечитать · Esc/n остаться",
		"danger.permission-mode.bypassPermissions": "Этот режим пропускает запросы разрешений и может допустить разрушительные действия.",
		"danger.sandbox.false":                     "Отключение sandbox убирает важный слой изоляции команд.",
		"danger.sandbox-unsandboxed.true":          "Команды вне sandbox получают доступ к ресурсам за пределами изоляции.",
		"tiny.text":                                "Терминал слишком мал.\nУвеличьте его хотя бы до 48×16.",
		"tiny.quit":                                "q выход",
		"status.cancelled":                         "Изменение отменено",
		"status.scope":                             "Редактируется уровень: %s",
		"status.inherit":                           "«%s» теперь наследует значение",
		"status.nothing_to_save":                   "На уровне «%s» нет изменений",
		"status.max_items":                         "«%s» принимает не более %d значений",
		"status.item_removed":                      "Элемент удалён",
		"status.empty":                             "Значение не может быть пустым; нажмите u для наследования",
		"status.duplicate":                         "Дубликаты не добавляются",
		"status.list_updated":                      "Список изменён",
		"status.staged":                            "«%s» изменено",
		"status.conflict":                          "Сохранение заблокировано: файл изменился на диске. Перечитайте его.",
		"status.save_failed":                       "Ошибка сохранения: %s",
		"status.saved":                             "Сохранено. Перезапустите активные сессии Claude Code.",
		"status.reload_failed":                     "Ошибка чтения: %s",
		"status.reloaded":                          "Настройки перечитаны с диска",

		"spec.main-model.label":          "Основная модель",
		"spec.subagent-model.label":      "Модель сабагентов",
		"spec.advisor-model.label":       "Модель advisor",
		"spec.fallback-models.label":     "Резервные модели",
		"spec.effort.label":              "Уровень рассуждения",
		"spec.always-thinking.label":     "Расширенное мышление",
		"spec.auto-compact.label":        "Автосжатие",
		"spec.agent-teams.label":         "Команды агентов",
		"spec.teammate-mode.label":       "Отображение агентов",
		"spec.agent-view.label":          "Отключить agent view",
		"spec.permission-mode.label":     "Режим разрешений",
		"spec.permission-allow.label":    "Разрешать",
		"spec.permission-ask.label":      "Всегда спрашивать",
		"spec.permission-deny.label":     "Запрещать",
		"spec.sandbox.label":             "Sandbox",
		"spec.sandbox-auto-allow.label":  "Разрешать Bash в sandbox",
		"spec.sandbox-unsandboxed.label": "Разрешить без sandbox",
		"spec.checkpointing.label":       "Контрольные точки файлов",
		"spec.theme.label":               "Тема Claude Code",
		"spec.tui.label":                 "TUI Claude Code",
		"spec.view.label":                "Вид истории",
		"spec.output-style.label":        "Стиль ответов",
		"spec.editor-mode.label":         "Режим редактора",
		"spec.ui-language.label":         "Язык интерфейса",
		"spec.language.label":            "Язык ответов Claude",
		"spec.reduced-motion.label":      "Меньше анимации",
		"spec.auto-memory.label":         "Автопамять",
		"spec.git-instructions.label":    "Git-инструкции",
		"spec.updates.label":             "Канал обновлений",

		"spec.main-model.description":          "Модель для новых сессий Claude Code. Стабильные алиасы автоматически следуют версии вашего провайдера.",
		"spec.subagent-model.description":      "Модель всех сабагентов и agent teams. Имеет приоритет над моделью отдельного агента.",
		"spec.advisor-model.description":       "Модель, которую использует серверный advisor.",
		"spec.fallback-models.description":     "Резервная цепочка до трёх уникальных моделей.",
		"spec.effort.description":              "Постоянный уровень адаптивного рассуждения для поддерживаемых моделей.",
		"spec.always-thinking.description":     "Включать расширенное мышление по умолчанию.",
		"spec.auto-compact.description":        "Автоматически сжимать диалог рядом с лимитом контекста.",
		"spec.agent-teams.description":         "Включить экспериментальные команды агентов.",
		"spec.teammate-mode.description":       "Как отображать участников команды агентов.",
		"spec.agent-view.description":          "Отключить фоновых агентов и экран управления агентами.",
		"spec.permission-mode.description":     "Поведение разрешений инструментов для новых сессий.",
		"spec.permission-allow.description":    "Правила, разрешённые без подтверждения.",
		"spec.permission-ask.description":      "Правила, которые всегда требуют подтверждения.",
		"spec.permission-deny.description":     "Всегда запрещённые правила инструментов.",
		"spec.sandbox.description":             "Запускать поддерживаемые команды в изолированном sandbox.",
		"spec.sandbox-auto-allow.description":  "Автоматически разрешать Bash, когда Claude Code может изолировать команду.",
		"spec.sandbox-unsandboxed.description": "Разрешить командам запрашивать запуск вне sandbox.",
		"spec.checkpointing.description":       "Сохранять снимки файлов для восстановления через /rewind.",
		"spec.theme.description":               "Цветовая тема Claude Code.",
		"spec.tui.description":                 "Полноэкранный или классический интерфейс Claude Code.",
		"spec.view.description":                "Количество деталей инструментов в истории.",
		"spec.output-style.description":        "Встроенный или пользовательский стиль ответов.",
		"spec.editor-mode.description":         "Клавиши обычного редактора или Vim.",
		"spec.ui-language.description":         "Язык Claude Configurator. Режим «Авто» следует языку операционной системы.",
		"spec.language.description":            "Предпочтительный язык ответов Claude, диктовки и заголовков сессий.",
		"spec.reduced-motion.description":      "Уменьшить необязательную терминальную анимацию.",
		"spec.auto-memory.description":         "Разрешить Claude Code сохранять полезный контекст проекта.",
		"spec.git-instructions.description":    "Добавлять встроенные инструкции рабочего процесса Git.",
		"spec.updates.description":             "Канал автоматических обновлений Claude Code.",

		"spec.main-model.purpose":          "Определяет баланс качества, скорости и стоимости в обычной работе без отдельного флага при каждом запуске.",
		"spec.subagent-model.purpose":      "Позволяет оставить сильную модель в основном чате, а делегированные задачи отдать более быстрой модели — или всем агентам дать возможности Fable.",
		"spec.advisor-model.purpose":       "Отделяет модель, которая проверяет сложное решение, от модели, выполняющей основную работу.",
		"spec.fallback-models.purpose":     "Помогает продолжить работу, если основная модель не может обслужить запрос и Claude Code требуется разрешённая замена.",
		"spec.effort.purpose":              "Низкие уровни быстрее и дешевле; высокие тратят больше рассуждений на сложные задачи.",
		"spec.always-thinking.purpose":     "Повышает качество сложных рассуждений, если дополнительная задержка и расход токенов приемлемы.",
		"spec.auto-compact.purpose":        "Не даёт длинной сессии упереться в лимит контекста, но итоговое резюме может потерять детали.",
		"spec.agent-teams.purpose":         "Полезно, когда независимые части задачи можно выполнять параллельно; экспериментальное поведение ещё может меняться.",
		"spec.teammate-mode.purpose":       "Оставьте агентов внутри процесса для простого терминала или вынесите в панели, чтобы наблюдать за каждым отдельно.",
		"spec.agent-view.purpose":          "Отключайте только если интерфейс фоновых агентов мешает или несовместим с терминалом.",
		"spec.permission-mode.purpose":     "Задаёт баланс скорости и безопасности: когда Claude обязан спросить разрешение перед действием.",
		"spec.permission-allow.purpose":    "Убирает лишние подтверждения для команд и путей, которым вы уже доверяете.",
		"spec.permission-ask.purpose":      "Оставляет чувствительные действия под ручным контролем, даже если более широкое правило их разрешает.",
		"spec.permission-deny.purpose":     "Блокирует команды, инструменты и пути, к которым Claude не должен получать доступ.",
		"spec.sandbox.purpose":             "Ограничивает ущерб от неожиданной shell-команды за счёт изоляции файловой системы и сети.",
		"spec.sandbox-auto-allow.purpose":  "Ускоряет безопасные сценарии, не убирая границу изоляции.",
		"spec.sandbox-unsandboxed.purpose": "Нужно только инструментам, которые не работают в изоляции; такие команды получают более широкий доступ к системе.",
		"spec.checkpointing.purpose":       "Позволяет восстановить ошибочное изменение через /rewind, даже если Git-коммита ещё нет.",
		"spec.ui-language.purpose":         "Режим «Авто» делает конфигуратор понятным на разных компьютерах без отдельной настройки языка.",
		"spec.theme.purpose":               "Помогает сохранить читаемость на светлом или тёмном фоне и выбрать доступную либо терминальную палитру.",
		"spec.tui.purpose":                 "Полноэкранный режим уменьшает мерцание; классический лучше совместим с обычным scrollback терминала.",
		"spec.view.purpose":                "Убирает шум для сосредоточенной работы или показывает больше деталей при отладке поведения агентов.",
		"spec.output-style.purpose":        "Меняет способ подачи ответа — кратко, объяснительно или учебно — без замены модели.",
		"spec.editor-mode.purpose":         "Выбирайте Vim, если модальное редактирование соответствует вашей привычной навигации.",
		"spec.language.purpose":            "Не приходится повторять требование к языку в каждом новом запросе.",
		"spec.reduced-motion.purpose":      "Повышает доступность и делает медленные или удалённые терминалы спокойнее.",
		"spec.auto-memory.purpose":         "Переносит полезные знания о проекте в следующие сессии без повторного объяснения.",
		"spec.git-instructions.purpose":    "Оставьте включённым, если проект не заменяет встроенный Git-процесс собственными правилами.",
		"spec.updates.purpose":             "Stable предсказуемее; latest раньше получает новые функции и исправления Claude Code.",
	},
	languageZH: {
		"scope.global":  "全局",
		"scope.project": "项目",
		"scope.local":   "本地",

		"category.Models":                  "模型",
		"category.Reasoning":               "推理",
		"category.Agents":                  "代理",
		"category.Permissions":             "权限",
		"category.Safety":                  "安全",
		"category.Interface":               "界面",
		"category.Behavior":                "行为",
		"category.Models.description":      "选择主模型、子代理、advisor 和后备模型。",
		"category.Reasoning.description":   "控制推理深度、思考和自动压缩。",
		"category.Agents.description":      "配置代理团队和后台代理显示方式。",
		"category.Permissions.description": "定义不同配置层级的工具权限。",
		"category.Safety.description":      "管理沙箱隔离和文件恢复保护。",
		"category.Interface.description":   "调整 Claude Code 和 Claude Configurator 的显示。",
		"category.Behavior.description":    "设置记忆、Git 指令和更新频道。",

		"option.inherit":                           "默认 / 继承",
		"option.custom":                            "自定义模型 ID…",
		"option.default":                           "账户默认",
		"option.best":                              "最佳可用模型",
		"option.fable":                             "Fable 5 · 推荐别名 · 1M",
		"option.sonnet":                            "Sonnet · 最新",
		"option.opus":                              "Opus · 最新",
		"option.haiku":                             "Haiku · 最新",
		"option.sonnet[1m]":                        "Sonnet · 1M 上下文",
		"option.opus[1m]":                          "Opus · 1M 上下文",
		"option.opusplan":                          "Opus 规划 → Sonnet 实现",
		"option.claude-fable-5[1m]":                "Fable 5 · 固定 ID · 1M",
		"option.claude-sonnet-5":                   "Sonnet 5 · 固定 ID",
		"option.theme.auto":                        "自动 · 跟随终端",
		"option.theme.dark":                        "深色",
		"option.theme.light":                       "浅色",
		"option.theme.dark-daltonized":             "深色 · 色觉友好",
		"option.theme.light-daltonized":            "浅色 · 色觉友好",
		"option.theme.dark-ansi":                   "深色 · 终端 ANSI 配色",
		"option.theme.light-ansi":                  "浅色 · 终端 ANSI 配色",
		"option.theme.custom":                      "自定义主题 · %s",
		"option.ui-language.auto":                  "自动 · 系统语言",
		"option.ui-language.en":                    "English",
		"option.ui-language.ru":                    "Русский",
		"option.ui-language.zh-CN":                 "简体中文",
		"app.source":                               "应用",
		"app.language.resolved":                    "自动识别为：%s",
		"app.language.saved":                       "界面语言已更新",
		"app.language.save_failed":                 "无法保存界面语言：%s",
		"header.staged":                            "● 已修改",
		"header.subtitle":                          "无需手动编辑 JSON 即可配置 Claude Code",
		"action.open":                              "打开",
		"action.edit":                              "修改",
		"action.inherit":                           "重置并继承",
		"action.save":                              "保存",
		"action.save_count":                        "保存 · %d",
		"action.search":                            "搜索",
		"action.scope":                             "层级",
		"action.help":                              "快捷键",
		"action.quit":                              "退出",
		"action.back":                              "返回",
		"footer.hint.categories":                   "↑↓ 选择分类 · 所有更改会在保存前保持暂存",
		"footer.hint.settings":                     "↑↓ 选择 · Space 切换 · U 删除当前层级值 · ↳ 表示继承",
		"panel.categories":                         "主菜单",
		"panel.settings":                           "设置",
		"panel.detail":                             "详情",
		"breadcrumb.menu":                          "主菜单",
		"breadcrumb.search":                        "搜索：%s",
		"category.settings_count":                  "%d 项设置",
		"empty.no_matches":                         "没有匹配的设置",
		"value.inherit":                            "继承",
		"value.claude_default":                     "Claude 默认",
		"value.items":                              "%d 项",
		"value.on":                                 "开启",
		"value.off":                                "关闭",
		"detail.this_scope":                        "当前层级",
		"detail.effective":                         "生效值",
		"detail.what":                              "它控制什么",
		"detail.why":                               "为什么需要它",
		"detail.key":                               "JSON 键 · %s",
		"detail.inherit_hint":                      "当前层级未保存值，正在使用继承结果；按 Enter 可创建覆盖值。",
		"detail.source":                            "来源：%s",
		"detail.suggestions":                       "选项：%s",
		"detail.permissions_merge":                 "权限数组会跨层级合并。",
		"detail.app_storage":                       "仅供 Claude Configurator 使用：\n%s",
		"search.title":                             "搜索设置",
		"search.description":                       "匹配名称、JSON 路径和说明。",
		"editor.custom_model":                      "自定义模型 ID",
		"editor.edit":                              "编辑 %s",
		"editor.apply":                             "Enter 应用 · Esc 取消 · ←→ 移动",
		"choice.help":                              "↑↓ 选择 · Enter 应用 · Esc 取消",
		"choice.inherit_help":                      "“默认 / 继承”会删除“%s”层级中的键，而不是写入一个伪默认值。",
		"list.empty":                               "当前层级没有值",
		"list.help":                                "a 添加 · Enter/e 编辑 · d 删除 · Esc 完成",
		"list.maximum":                             "最多：%d 项",
		"diff.more":                                "… 还有 %d 项更改",
		"diff.help":                                "Enter/y/s 保存 · Esc/n 取消",
		"diff.title":                               "保存“%s”设置？",
		"help.title":                               "键盘帮助",
		"help.close":                               "Esc 或 ? 关闭",
		"confirm.danger.title":                     "危险设置",
		"confirm.danger.help":                      "Enter/y 应用 · Esc/n 取消",
		"confirm.quit.title":                       "放弃暂存的更改？",
		"confirm.quit.text":                        "一个或多个层级存在未保存的更改。",
		"confirm.quit.help":                        "Enter/y 放弃 · Esc/n 返回",
		"confirm.reload.title":                     "从磁盘重新加载？",
		"confirm.reload.text":                      "所有暂存的更改都将丢失。",
		"confirm.reload.help":                      "Enter/y 重载 · Esc/n 返回",
		"danger.permission-mode.bypassPermissions": "此模式会跳过权限提示，并可能允许破坏性操作。",
		"danger.sandbox.false":                     "禁用沙箱会移除重要的命令隔离层。",
		"danger.sandbox-unsandboxed.true":          "非沙箱命令可以访问隔离边界之外的资源。",
		"tiny.text":                                "终端太小。\n请至少调整到 48×16。",
		"tiny.quit":                                "q 退出",
		"status.cancelled":                         "更改已取消",
		"status.scope":                             "正在编辑%s设置",
		"status.inherit":                           "“%s”现在继承低层级值",
		"status.nothing_to_save":                   "“%s”没有需要保存的更改",
		"status.max_items":                         "“%s”最多接受 %d 项",
		"status.item_removed":                      "项目已删除",
		"status.empty":                             "值不能为空；按 u 使用继承值",
		"status.duplicate":                         "已忽略重复项目",
		"status.list_updated":                      "列表已更新",
		"status.staged":                            "“%s”已暂存",
		"status.conflict":                          "保存被阻止：文件已在磁盘上更改，请先重新加载。",
		"status.save_failed":                       "保存失败：%s",
		"status.saved":                             "已保存。请重启正在运行的 Claude Code 会话。",
		"status.reload_failed":                     "重新加载失败：%s",
		"status.reloaded":                          "已从磁盘重新加载设置",

		"spec.main-model.label":          "主模型",
		"spec.subagent-model.label":      "子代理模型",
		"spec.advisor-model.label":       "Advisor 模型",
		"spec.fallback-models.label":     "后备模型",
		"spec.effort.label":              "推理强度",
		"spec.always-thinking.label":     "扩展思考",
		"spec.auto-compact.label":        "自动压缩",
		"spec.agent-teams.label":         "代理团队",
		"spec.teammate-mode.label":       "队友显示",
		"spec.agent-view.label":          "禁用代理视图",
		"spec.permission-mode.label":     "默认权限模式",
		"spec.permission-allow.label":    "允许规则",
		"spec.permission-ask.label":      "询问规则",
		"spec.permission-deny.label":     "拒绝规则",
		"spec.sandbox.label":             "沙箱",
		"spec.sandbox-auto-allow.label":  "自动允许沙箱 Bash",
		"spec.sandbox-unsandboxed.label": "允许非沙箱命令",
		"spec.checkpointing.label":       "文件检查点",
		"spec.theme.label":               "Claude Code 主题",
		"spec.tui.label":                 "Claude Code 界面",
		"spec.view.label":                "记录视图",
		"spec.output-style.label":        "输出风格",
		"spec.editor-mode.label":         "编辑模式",
		"spec.ui-language.label":         "界面语言",
		"spec.language.label":            "Claude 回复语言",
		"spec.reduced-motion.label":      "减少动画",
		"spec.auto-memory.label":         "自动记忆",
		"spec.git-instructions.label":    "Git 指令",
		"spec.updates.label":             "更新频道",

		"spec.main-model.description":          "新 Claude Code 会话使用的模型。稳定别名会跟随提供商的最新可用版本。",
		"spec.subagent-model.description":      "所有子代理和代理团队使用的模型，优先于单个代理配置。",
		"spec.advisor-model.description":       "服务器端 advisor 使用的模型。",
		"spec.fallback-models.description":     "最多三个唯一模型组成的后备链。",
		"spec.effort.description":              "支持模型的持久自适应推理强度。",
		"spec.always-thinking.description":     "默认启用扩展思考。",
		"spec.auto-compact.description":        "接近上下文限制时自动压缩对话。",
		"spec.agent-teams.description":         "启用实验性的代理团队功能。",
		"spec.teammate-mode.description":       "代理团队成员的显示方式。",
		"spec.agent-view.description":          "禁用后台代理和代理管理视图。",
		"spec.permission-mode.description":     "新会话的默认工具权限行为。",
		"spec.permission-allow.description":    "无需提示即可允许的工具规则。",
		"spec.permission-ask.description":      "始终要求确认的工具规则。",
		"spec.permission-deny.description":     "始终拒绝的工具规则。",
		"spec.sandbox.description":             "在 Claude Code 沙箱中运行支持的命令。",
		"spec.sandbox-auto-allow.description":  "命令可被沙箱隔离时自动允许 Bash。",
		"spec.sandbox-unsandboxed.description": "允许命令请求在沙箱外运行。",
		"spec.checkpointing.description":       "保存文件快照，以便通过 /rewind 恢复。",
		"spec.theme.description":               "Claude Code 的配色主题。",
		"spec.tui.description":                 "选择全屏或经典 Claude Code 界面。",
		"spec.view.description":                "记录中显示的工具详情数量。",
		"spec.output-style.description":        "内置或自定义的助手输出风格。",
		"spec.editor-mode.description":         "普通或 Vim 输入按键。",
		"spec.ui-language.description":         "Claude Configurator 使用的语言；自动模式跟随操作系统。",
		"spec.language.description":            "Claude 回复、听写和会话标题的首选语言。",
		"spec.reduced-motion.description":      "减少非必要的终端动画。",
		"spec.auto-memory.description":         "允许 Claude Code 自动保存有用的项目上下文。",
		"spec.git-instructions.description":    "包含 Claude Code 内置的 Git 工作流说明。",
		"spec.updates.description":             "Claude Code 自动更新频道。",

		"spec.main-model.purpose":          "决定日常工作的能力、速度和成本，无需每次启动都传入模型参数。",
		"spec.subagent-model.purpose":      "主会话可以保留更强模型，同时让委派任务使用更快模型；也可以让所有代理使用 Fable 级能力。",
		"spec.advisor-model.purpose":       "将审查困难决策的模型与执行主要工作的模型分开。",
		"spec.fallback-models.purpose":     "首选模型无法处理请求时，让 Claude Code 改用另一个获准模型继续工作。",
		"spec.effort.purpose":              "较低级别更快、更省；较高级别会为复杂任务投入更多推理。",
		"spec.always-thinking.purpose":     "在可以接受额外延迟和令牌消耗时，提高复杂推理质量。",
		"spec.auto-compact.purpose":        "避免长会话触及上下文上限，但压缩摘要可能遗漏细节。",
		"spec.agent-teams.purpose":         "适合并行处理相互独立的任务部分；实验性行为仍可能变化。",
		"spec.teammate-mode.purpose":       "简单终端可使用进程内显示；需要分别观察代理时可使用独立面板。",
		"spec.agent-view.purpose":          "仅当后台代理界面造成干扰或与终端不兼容时关闭。",
		"spec.permission-mode.purpose":     "在效率与安全之间取舍，决定 Claude 执行动作前何时必须询问。",
		"spec.permission-allow.purpose":    "对已经信任的命令和路径减少重复确认。",
		"spec.permission-ask.purpose":      "即使更宽泛的规则允许，也让敏感动作始终由你确认。",
		"spec.permission-deny.purpose":     "阻止 Claude 访问绝不应使用的命令、工具或路径。",
		"spec.sandbox.purpose":             "通过隔离文件系统和网络，限制意外 shell 命令可能造成的损害。",
		"spec.sandbox-auto-allow.purpose":  "在不移除隔离边界的前提下加快安全工作流。",
		"spec.sandbox-unsandboxed.purpose": "只适用于无法在隔离环境中工作的工具；这些命令会获得更广泛的系统访问。",
		"spec.checkpointing.purpose":       "即使尚未创建 Git 提交，也能通过 /rewind 恢复错误编辑。",
		"spec.ui-language.purpose":         "自动模式会跟随不同电脑的系统语言，无需单独维护界面语言。",
		"spec.theme.purpose":               "在明暗背景下保持可读性，并可选择色觉友好或终端原生配色。",
		"spec.tui.purpose":                 "全屏模式减少闪烁；经典模式更兼容终端原生回滚记录。",
		"spec.view.purpose":                "专注工作时减少噪音，调试代理行为时显示更多细节。",
		"spec.output-style.purpose":        "无需更换模型，即可选择简洁、解释型或学习型的回答方式。",
		"spec.editor-mode.purpose":         "如果你习惯 Vim 的模态导航，可选择 Vim 模式。",
		"spec.language.purpose":            "无需在每个新提示中重复指定回复语言。",
		"spec.reduced-motion.purpose":      "提升可访问性，并让较慢或远程终端更平静。",
		"spec.auto-memory.purpose":         "把有用的项目知识带到后续会话，无需重复说明。",
		"spec.git-instructions.purpose":    "如果项目没有自己的 Git 工作流规则，建议保持开启。",
		"spec.updates.purpose":             "stable 更可预测；latest 更早获得 Claude Code 新功能和修复。",
	},
}
