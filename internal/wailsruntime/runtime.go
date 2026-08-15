package wailsruntime

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ErrUnavailable is returned when a Wails-dependent operation is used outside
// the GUI runtime, such as from a unit test or the standalone CLI.
var ErrUnavailable = errors.New("Wails application runtime is unavailable")

const wailsDialogCancelledMessage = "cancelled by user"

// AutostartLaunchArgument lets LunaBox distinguish a login launch from a
// normal user-initiated launch.
const AutostartLaunchArgument = "--autostart"

// Runtime is the small subset of the Wails v3 application/window API used by
// LunaBox services. Services receive it explicitly instead of resolving global
// application state or using the context-based Wails v2 runtime shape.
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

// OpenDialogOptions is the service-facing subset of Wails v3 open-dialog
// options. Wails v3 does not support preselecting a filename in an open dialog.
type OpenDialogOptions struct {
	Directory                       string
	Title                           string
	Filters                         []FileFilter
	ShowHiddenFiles                 bool
	ResolvesAliases                 bool
	TreatsFilePackagesAsDirectories bool
}

// SaveDialogOptions is the service-facing subset of Wails v3 save-dialog
// options.
type SaveDialogOptions struct {
	Directory                       string
	Filename                        string
	Title                           string
	Filters                         []FileFilter
	ShowHiddenFiles                 bool
	TreatsFilePackagesAsDirectories bool
}

type applicationRuntime struct {
	app    *application.App
	window *application.WebviewWindow
}

// New creates a runtime adapter backed by the Wails v3 application and main
// window objects.
func New(app *application.App, window *application.WebviewWindow) Runtime {
	return &applicationRuntime{app: app, window: window}
}

// Unavailable returns a stateless runtime for non-GUI construction paths.
// Event/window operations are harmless no-ops; operations with a result return
// ErrUnavailable.
func Unavailable() Runtime {
	return unavailableRuntime{}
}

func (r *applicationRuntime) Emit(name string, data ...any) bool {
	if r == nil || r.app == nil {
		return false
	}
	return r.app.Event.Emit(name, data...)
}

func (r *applicationRuntime) OpenURL(targetURL string) error {
	if r == nil || r.app == nil {
		return ErrUnavailable
	}
	return r.app.Browser.OpenURL(targetURL)
}

func (r *applicationRuntime) SetAutostart(enabled bool) error {
	if r == nil || r.app == nil || r.app.Autostart == nil {
		return ErrUnavailable
	}
	if !enabled {
		return r.app.Autostart.Disable()
	}
	return r.app.Autostart.EnableWithOptions(application.AutostartOptions{
		Arguments: []string{AutostartLaunchArgument},
	})
}

func (r *applicationRuntime) RestoreWindow() {
	if r != nil && r.window != nil {
		r.window.Restore()
	}
}

func (r *applicationRuntime) ShowWindow() {
	if r != nil && r.window != nil {
		r.window.Show()
	}
}

func (r *applicationRuntime) OpenFile(options OpenDialogOptions) (string, error) {
	if r == nil || r.app == nil {
		return "", ErrUnavailable
	}

	dialog := r.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseFiles:                  true,
		CanChooseDirectories:            false,
		CanCreateDirectories:            false,
		ShowHiddenFiles:                 options.ShowHiddenFiles,
		ResolvesAliases:                 options.ResolvesAliases,
		TreatsFilePackagesAsDirectories: options.TreatsFilePackagesAsDirectories,
		Title:                           options.Title,
		Directory:                       options.Directory,
		Filters:                         applicationFileFilters(options.Filters),
		Window:                          r.window,
	})
	return normalizeDialogSelection(dialog.PromptForSingleSelection())
}

func (r *applicationRuntime) OpenDirectory(options OpenDialogOptions) (string, error) {
	if r == nil || r.app == nil {
		return "", ErrUnavailable
	}

	dialog := r.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseFiles:                  false,
		CanChooseDirectories:            true,
		CanCreateDirectories:            true,
		ShowHiddenFiles:                 options.ShowHiddenFiles,
		ResolvesAliases:                 options.ResolvesAliases,
		TreatsFilePackagesAsDirectories: options.TreatsFilePackagesAsDirectories,
		Title:                           options.Title,
		Directory:                       options.Directory,
		Filters:                         applicationFileFilters(options.Filters),
		Window:                          r.window,
	})
	return normalizeDialogSelection(dialog.PromptForSingleSelection())
}

func (r *applicationRuntime) SaveFile(options SaveDialogOptions) (string, error) {
	if r == nil || r.app == nil {
		return "", ErrUnavailable
	}

	dialog := r.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		CanCreateDirectories:            true,
		ShowHiddenFiles:                 options.ShowHiddenFiles,
		TreatsFilePackagesAsDirectories: options.TreatsFilePackagesAsDirectories,
		Title:                           options.Title,
		Directory:                       options.Directory,
		Filename:                        options.Filename,
		Filters:                         applicationFileFilters(options.Filters),
		Window:                          r.window,
	})
	return normalizeDialogSelection(dialog.PromptForSingleSelection())
}

func applicationFileFilters(filters []FileFilter) []application.FileFilter {
	result := make([]application.FileFilter, 0, len(filters))
	for _, filter := range filters {
		result = append(result, application.FileFilter{
			DisplayName: filter.DisplayName,
			Pattern:     filter.Pattern,
		})
	}
	return result
}

func normalizeDialogSelection(selection string, err error) (string, error) {
	if err == nil {
		return selection, nil
	}

	// Wails v3 alpha keeps its cancellation sentinel in an internal package,
	// so the adapter cannot use errors.Is with the original value.
	for current := err; current != nil; current = errors.Unwrap(current) {
		if current.Error() == wailsDialogCancelledMessage {
			return "", nil
		}
	}

	return "", err
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
