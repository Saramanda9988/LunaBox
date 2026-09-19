package steam

// LocalGame represents an installed game discovered from a Steam library.
type LocalGame struct {
	AppID        string   `json:"app_id"`
	Name         string   `json:"name"`
	InstallDir   string   `json:"install_dir"`
	LibraryPath  string   `json:"library_path"`
	ManifestPath string   `json:"manifest_path"`
	SizeOnDisk   int64    `json:"size_on_disk"`
	StateFlags   int      `json:"state_flags"`
	Executables  []string `json:"executables"`
	SelectedExe  string   `json:"selected_exe"`
	ProtonPrefix string   `json:"proton_prefix"`
}
