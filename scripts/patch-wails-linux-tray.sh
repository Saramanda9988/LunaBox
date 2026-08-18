#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT_DIR"

if [[ "$(uname -s)" != "Linux" ]]; then
    exit 0
fi

check_tool() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "ERROR: $1 was not found in PATH." >&2
        exit 1
    }
}

check_tool python3

if [[ -n "${LUNABOX_WAILS_MODULE_DIR:-}" ]]; then
    module_version="${LUNABOX_WAILS_MODULE_VERSION:-local}"
    module_dir="$(cd "$LUNABOX_WAILS_MODULE_DIR" && pwd -P)"
else
    check_tool go
    module_version="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)"
    if [[ -z "$module_version" || "$module_version" == "<nil>" ]]; then
        echo "ERROR: failed to resolve github.com/wailsapp/wails/v3 module version." >&2
        exit 1
    fi
    go mod download github.com/wailsapp/wails/v3
    module_dir="$(go list -m -f '{{.Dir}}' github.com/wailsapp/wails/v3)"
    if [[ -z "$module_dir" || "$module_dir" == "<nil>" ]]; then
        echo "ERROR: failed to resolve github.com/wailsapp/wails/v3 module directory." >&2
        exit 1
    fi
    module_dir="$(cd "$module_dir" && pwd -P)"
fi

systemtray_go="$module_dir/pkg/application/systemtray.go"
linux_go="$module_dir/pkg/application/systemtray_linux.go"

for path in "$systemtray_go" "$linux_go"; do
    if [[ ! -f "$path" || -L "$path" ]]; then
        echo "ERROR: Wails source file not found: $path" >&2
        exit 1
    fi
done

chmod u+w "$systemtray_go" "$linux_go"

python3 - "$systemtray_go" "$linux_go" <<'PY'
from pathlib import Path
import sys

systemtray_go = Path(sys.argv[1])
linux_go = Path(sys.argv[2])


def read_source(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def write_source(path: Path, text: str) -> None:
    path.write_text(text, encoding="utf-8")


def replace_once(path: Path, old: str, new: str) -> bool:
    text = read_source(path)
    if new in text:
        if text.count(new) != 1:
            raise SystemExit(f"ERROR: patched source block is duplicated in {path}")
        return False
    old_count = text.count(old)
    if old_count != 1:
        raise SystemExit(f"ERROR: expected source block not found in {path}")
    write_source(path, text.replace(old, new, 1))
    return True


def replace_if_present(path: Path, old: str, new: str) -> bool:
    text = read_source(path)
    if new in text:
        if text.count(new) != 1:
            raise SystemExit(f"ERROR: patched source block is duplicated in {path}")
        return False
    old_count = text.count(old)
    if old_count == 0:
        return False
    if old_count != 1:
        raise SystemExit(f"ERROR: source block is ambiguous in {path}")
    write_source(path, text.replace(old, new, 1))
    return True


changed = False

changed |= replace_if_present(
    systemtray_go,
    '''\t// LunaBox patch: keep Linux tray menus host-rendered through StatusNotifierItem.Menu.
\t// Wails v3 beta.5 OpenMenu is not implemented on Linux, so installing ShowMenu
\t// as the default right-click handler eats the tray host's context-menu event.
\tif s.rightClickHandler == nil && hasMenu && runtime.GOOS != "linux" {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
    '''\t// LunaBox patch: on Linux, leave ContextMenu unhandled so the tray host
\t// falls back to StatusNotifierItem.Menu and renders the exported DBusMenu.
\t// Wails v3 beta.5 installs ShowMenu by default, but OpenMenu is not
\t// implemented on Linux, which makes right-click look dead.
\tif s.rightClickHandler == nil && hasMenu && runtime.GOOS != "linux" {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
)

changed |= replace_if_present(
    systemtray_go,
    '''\tif s.rightClickHandler == nil && hasMenu && runtime.GOOS != "linux" {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
    '''\t// LunaBox patch: on Linux, leave ContextMenu unhandled so the tray host
\t// falls back to StatusNotifierItem.Menu and renders the exported DBusMenu.
\t// Wails v3 beta.5 installs ShowMenu by default, but OpenMenu is not
\t// implemented on Linux, which makes right-click look dead.
\tif s.rightClickHandler == nil && hasMenu && runtime.GOOS != "linux" {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
)

changed |= replace_once(
    systemtray_go,
    '''\tif s.rightClickHandler == nil && hasMenu {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
    '''\t// LunaBox patch: on Linux, leave ContextMenu unhandled so the tray host
\t// falls back to StatusNotifierItem.Menu and renders the exported DBusMenu.
\t// Wails v3 beta.5 installs ShowMenu by default, but OpenMenu is not
\t// implemented on Linux, which makes right-click look dead.
\tif s.rightClickHandler == nil && hasMenu && runtime.GOOS != "linux" {
\t\ts.rightClickHandler = s.ShowMenu
\t}
''',
)

changed |= replace_once(
    linux_go,
    '''\timpl := &linuxSystemTray{
\t\tparent:         s,
\t\tid:             s.id,
\t\tlabel:          label,
\t\ticon:           s.icon,
\t\tmenu:           s.menu,
\t\ticonPosition:   s.iconPosition,
\t\tisTemplateIcon: s.isTemplateIcon,
\t\tquitChan:       make(chan struct{}),
\t}
''',
    '''\timpl := &linuxSystemTray{
\t\tparent:         s,
\t\tid:             s.id,
\t\tlabel:          label,
\t\ttooltip:        s.tooltip,
\t\ticon:           s.icon,
\t\tmenu:           s.menu,
\t\ticonPosition:   s.iconPosition,
\t\tisTemplateIcon: s.isTemplateIcon,
\t\tquitChan:       make(chan struct{}),
\t}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) setTooltip(_ string) {
\t// TBD
}
''',
    '''func (s *linuxSystemTray) setTooltip(tooltipText string) {
\ts.tooltip = tooltipText
\tif tooltipText == "" {
\t\ttooltipText = s.label
\t}
\tif s.props == nil {
\t\treturn
\t}
\tif err := s.props.Set("org.kde.StatusNotifierItem", "ToolTip", dbus.MakeVariant(tooltip{V2: tooltipText})); err != nil {
\t\tglobalApplication.error("systray error: failed to set ToolTip prop: %w", err)
\t}
}
''',
)

changed |= replace_if_present(
    linux_go,
    '''func (s *linuxSystemTray) openMenu() {
\t// Linux StatusNotifier hosts open the exported dbusmenu themselves.
}
''',
    '''func (s *linuxSystemTray) openMenu() {
\t// Linux tray hosts open the exported DBusMenu after ContextMenu returns.
}
''',
)

changed |= replace_if_present(
    linux_go,
    '''func (s *linuxSystemTray) openMenu() {
\t// LunaBox patch: Linux tray menu is opened by the tray host through
\t// StatusNotifierItem.Menu and com.canonical.dbusmenu. There is no app-side
\t// popup implementation in Wails v3 beta.5.
}
''',
    '''func (s *linuxSystemTray) openMenu() {
\t// Linux tray hosts open the exported DBusMenu after ContextMenu returns.
}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) openMenu() {
\t// FIXME: Emit com.canonical to open?
\tglobalApplication.info("systray error: openMenu not implemented on Linux")
}
''',
    '''func (s *linuxSystemTray) openMenu() {
\t// Linux tray hosts open the exported DBusMenu after ContextMenu returns.
}
''',
)

changed |= replace_if_present(
    linux_go,
    '''func (s *linuxSystemTray) ContextMenu(x int32, y int32) (err *dbus.Error) {
\ts.lastClickX = int(x)
\ts.lastClickY = int(y)
\treturn nil
}
''',
    '''func (s *linuxSystemTray) ContextMenu(x int32, y int32) (err *dbus.Error) {
\ts.lastClickX = int(x)
\ts.lastClickY = int(y)
\tglobalApplication.debug("systray ContextMenu called", "x", x, "y", y)
\tif s.parent.rightClickHandler == nil {
\t\treturn &dbus.ErrMsgUnknownMethod
\t}
\ts.parent.rightClickHandler()
\treturn nil
}
''',
)

changed |= replace_if_present(
    linux_go,
    '''func (s *linuxSystemTray) ContextMenu(x int32, y int32) (err *dbus.Error) {
\ts.lastClickX = int(x)
\ts.lastClickY = int(y)
\tglobalApplication.debug("systray ContextMenu called", "x", x, "y", y)
\tif s.parent.rightClickHandler != nil {
\t\ts.parent.rightClickHandler()
\t}
\treturn nil
}
''',
    '''func (s *linuxSystemTray) ContextMenu(x int32, y int32) (err *dbus.Error) {
\ts.lastClickX = int(x)
\ts.lastClickY = int(y)
\tglobalApplication.debug("systray ContextMenu called", "x", x, "y", y)
\tif s.parent.rightClickHandler == nil {
\t\treturn &dbus.ErrMsgUnknownMethod
\t}
\ts.parent.rightClickHandler()
\treturn nil
}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) setLabel(label string) {
\ts.label = label

\tif err := s.props.Set("org.kde.StatusNotifierItem", "Title", dbus.MakeVariant(label)); err != nil {
\t\tglobalApplication.error("systray error: failed to set Title prop: %w", err)
\t\treturn
\t}

\tif s.conn == nil {
\t\treturn
\t}

\tif err := notifier.Emit(s.conn, &notifier.StatusNotifierItem_NewTitleSignal{
\t\tPath: itemPath,
\t\tBody: &notifier.StatusNotifierItem_NewTitleSignalBody{},
\t}); err != nil {
\t\tglobalApplication.error("systray error: failed to emit new title signal: %w", err)
\t\treturn
\t}

}
''',
    '''func (s *linuxSystemTray) setLabel(label string) {
\ts.label = label
\tif s.props == nil {
\t\treturn
\t}

\tif err := s.props.Set("org.kde.StatusNotifierItem", "Title", dbus.MakeVariant(label)); err != nil {
\t\tglobalApplication.error("systray error: failed to set Title prop: %w", err)
\t\treturn
\t}
\tif s.tooltip == "" {
\t\tif err := s.props.Set("org.kde.StatusNotifierItem", "ToolTip", dbus.MakeVariant(tooltip{V2: label})); err != nil {
\t\t\tglobalApplication.error("systray error: failed to set ToolTip prop: %w", err)
\t\t}
\t}

\tif s.conn == nil {
\t\treturn
\t}

\tif err := notifier.Emit(s.conn, &notifier.StatusNotifierItem_NewTitleSignal{
\t\tPath: itemPath,
\t\tBody: &notifier.StatusNotifierItem_NewTitleSignalBody{},
\t}); err != nil {
\t\tglobalApplication.error("systray error: failed to emit new title signal: %w", err)
\t\treturn
\t}

}
''',
)

changed |= replace_once(
    linux_go,
    '''func (s *linuxSystemTray) createPropSpec() map[string]map[string]*prop.Prop {
\tprops := map[string]*prop.Prop{
''',
    '''func (s *linuxSystemTray) createPropSpec() map[string]map[string]*prop.Prop {
\ttooltipText := s.tooltip
\tif tooltipText == "" {
\t\ttooltipText = s.label
\t}

\tprops := map[string]*prop.Prop{
''',
)

changed |= replace_once(
    linux_go,
    '''\t\t"ToolTip": {
\t\t\tValue:    tooltip{V2: s.label},
\t\t\tWritable: true,
\t\t\tEmit:     prop.EmitTrue,
\t\t\tCallback: nil,
\t\t},
''',
    '''\t\t"ToolTip": {
\t\t\tValue:    tooltip{V2: tooltipText},
\t\t\tWritable: true,
\t\t\tEmit:     prop.EmitTrue,
\t\t\tCallback: nil,
\t\t},
''',
)

changed |= replace_if_present(
    linux_go,
    '''\tif s.menu != nil {
\t\tprops["Menu"] = &prop.Prop{
\t\t\tValue:    dbus.ObjectPath(menuPath),
\t\t\tWritable: true,
\t\t\tEmit:     prop.EmitTrue,
\t\t\tCallback: nil,
\t\t}
\t}
''',
    '''\tprops["Menu"] = &prop.Prop{
\t\tValue:    dbus.ObjectPath(menuPath),
\t\tWritable: true,
\t\tEmit:     prop.EmitTrue,
\t\tCallback: nil,
\t}
''',
)

changed |= replace_if_present(
    linux_go,
    '''\tprops["Menu"] = &prop.Prop{
\t\tValue:    dbus.ObjectPath(menuPath),
\t\tWritable: false,
\t\tEmit:     prop.EmitTrue,
\t\tCallback: nil,
\t}
''',
    '''\tprops["Menu"] = &prop.Prop{
\t\tValue:    dbus.ObjectPath(menuPath),
\t\tWritable: true,
\t\tEmit:     prop.EmitTrue,
\t\tCallback: nil,
\t}
''',
)

changed |= replace_once(
    linux_go,
    '''\tcase "opened":
\t\tif s.parent.clickHandler != nil {
\t\t\ts.parent.clickHandler()
\t\t}
\t\tif s.parent.onMenuOpen != nil {
\t\t\ts.parent.onMenuOpen()
\t\t}
''',
    '''\tcase "opened":
\t\tif s.parent.onMenuOpen != nil {
\t\t\ts.parent.onMenuOpen()
\t\t}
''',
)

systemtray_text = read_source(systemtray_go)
linux_text = read_source(linux_go)
required_snippets = [
    (systemtray_go, systemtray_text, 'runtime.GOOS != "linux"'),
    (linux_go, linux_text, 'tooltip:        s.tooltip'),
    (linux_go, linux_text, 'func (s *linuxSystemTray) setTooltip(tooltipText string)'),
    (linux_go, linux_text, 'return &dbus.ErrMsgUnknownMethod'),
    (linux_go, linux_text, '''props["Menu"] = &prop.Prop{
\t\tValue:    dbus.ObjectPath(menuPath),
\t\tWritable: true,
'''),
    (linux_go, linux_text, '''\tcase "opened":
\t\tif s.parent.onMenuOpen != nil {
'''),
]
for path, text_value, snippet in required_snippets:
    if snippet not in text_value:
        raise SystemExit(f"ERROR: Wails Linux tray patch verification failed for {path}")

if changed:
    print("patched")
else:
    print("already patched")
PY

echo "Wails Linux tray patch applied for $module_version"
