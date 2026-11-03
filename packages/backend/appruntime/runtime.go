package appruntime

import "errors"

// ErrUnavailable is returned when a GUI-dependent operation is used outside
// the GUI runtime, such as from a unit test or the standalone CLI.
var ErrUnavailable = errors.New("application runtime is unavailable")

// AutostartLaunchArgument lets LunaBox distinguish a login launch from a
// normal user-initiated launch.
const AutostartLaunchArgument = "--autostart"

// Runtime is the small subset of the desktop application/window API used by
// LunaBox services. The desktop shell implements it (apps/desktop/host with
// MyGo); services receive it explicitly instead of resolving global
// application state.
type Runtime interface {
	Emit(name string, data ...any) bool
	OpenURL(targetURL string) error
	SetAutostart(enabled bool) error
	RestoreWindow()
	ShowWindow()
	OpenFile(options OpenDialogOptions) (string, error)
	OpenDirectory(options OpenDialogOptions) (string, error)
	SaveFile(options SaveDialogOptions) (string, error)
}

// FileFilter describes one native file-dialog filter.
type FileFilter struct {
	DisplayName string
	Pattern     string
}

// OpenDialogOptions is the service-facing subset of open-dialog options.
// Preselecting a filename in an open dialog is not supported.
type OpenDialogOptions struct {
	Directory                       string
	Title                           string
	Filters                         []FileFilter
	ShowHiddenFiles                 bool
	ResolvesAliases                 bool
	TreatsFilePackagesAsDirectories bool
}

// SaveDialogOptions is the service-facing subset of save-dialog options.
type SaveDialogOptions struct {
	Directory                       string
	Filename                        string
	Title                           string
	Filters                         []FileFilter
	ShowHiddenFiles                 bool
	TreatsFilePackagesAsDirectories bool
}

// Unavailable returns a stateless runtime for non-GUI construction paths.
// Event/window operations are harmless no-ops; operations with a result return
// ErrUnavailable.
func Unavailable() Runtime {
	return unavailableRuntime{}
}

type unavailableRuntime struct{}

func (unavailableRuntime) Emit(string, ...any) bool { return false }

func (unavailableRuntime) OpenURL(string) error { return ErrUnavailable }

func (unavailableRuntime) SetAutostart(bool) error { return ErrUnavailable }

func (unavailableRuntime) RestoreWindow() {}

func (unavailableRuntime) ShowWindow() {}

func (unavailableRuntime) OpenFile(OpenDialogOptions) (string, error) {
	return "", ErrUnavailable
}

func (unavailableRuntime) OpenDirectory(OpenDialogOptions) (string, error) {
	return "", ErrUnavailable
}

func (unavailableRuntime) SaveFile(SaveDialogOptions) (string, error) {
	return "", ErrUnavailable
}
