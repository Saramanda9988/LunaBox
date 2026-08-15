//go:build !darwin && !linux

package appconf

func detectDefaultCrossOverRunnerPath(config *AppConfig) bool {
	return false
}

func detectDefaultWineRunnerPath(config *AppConfig) bool {
	return false
}
