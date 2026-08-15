#import <AppKit/AppKit.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

uint32_t lunabox_frontmost_process_id(void) {
    @autoreleasepool {
        NSRunningApplication *application = NSWorkspace.sharedWorkspace.frontmostApplication;
        if (application == nil || application.processIdentifier <= 0) {
            return 0;
        }
        return (uint32_t)application.processIdentifier;
    }
}

char *lunabox_frontmost_bundle_path(void) {
    @autoreleasepool {
        NSURL *bundleURL = NSWorkspace.sharedWorkspace.frontmostApplication.bundleURL;
        const char *path = bundleURL.path.fileSystemRepresentation;
        if (path == NULL || path[0] == '\0') {
            return NULL;
        }
        return strdup(path);
    }
}
