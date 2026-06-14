//go:build darwin

package apptray

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa

void lunaboxTrayStart(const char *iconBytes, int iconLength);
void lunaboxTrayStop(void);
*/
import "C"

import "unsafe"

func Start(options Options) {
	setCallbacks(options.Callbacks)

	var iconPtr *C.char
	iconBytes := options.DarwinIcon
	if len(iconBytes) == 0 {
		iconBytes = options.AppIcon
	}
	if len(iconBytes) == 0 {
		iconBytes = options.Icon
	}
	if len(iconBytes) > 0 {
		iconPtr = (*C.char)(unsafe.Pointer(&iconBytes[0]))
	}
	C.lunaboxTrayStart(iconPtr, C.int(len(iconBytes)))
}

func Stop() {
	C.lunaboxTrayStop()
}

//export lunaboxTrayReady
func lunaboxTrayReady() {
	notifyReady()
}

//export lunaboxTrayExit
func lunaboxTrayExit() {
	notifyExit()
}

//export lunaboxTrayShowMainWindow
func lunaboxTrayShowMainWindow() {
	go notifyNativeMainWindowDidShow()
}

//export lunaboxTrayQuitApplication
func lunaboxTrayQuitApplication() {
	requestQuit()
}
