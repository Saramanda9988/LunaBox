//go:build darwin

package apptray

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <stdlib.h>

extern void lunaboxTrayReady(void);
extern void lunaboxTrayExit(void);
extern void lunaboxTrayShowMainWindow(void);
extern void lunaboxTrayQuitApplication(void);

@interface LunaBoxTrayTarget : NSObject
- (void)showMainWindow:(id)sender;
- (void)quitApplication:(id)sender;
@end

@implementation LunaBoxTrayTarget
- (void)showMainWindow:(id)sender {
	for (NSWindow *window in [NSApp windows]) {
		if ([window canBecomeKeyWindow]) {
			[window deminiaturize:nil];
			[window makeKeyAndOrderFront:nil];
			break;
		}
	}
	[NSApp unhide:nil];
	[NSApp activateIgnoringOtherApps:YES];
	lunaboxTrayShowMainWindow();
}

- (void)quitApplication:(id)sender {
	lunaboxTrayQuitApplication();
}
@end

static NSStatusItem *lunaboxStatusItem = nil;
static LunaBoxTrayTarget *lunaboxTrayTarget = nil;

void lunaboxTrayStart(const char *iconBytes, int iconLength) {
	NSData *iconData = nil;
	if (iconBytes != NULL && iconLength > 0) {
		iconData = [NSData dataWithBytes:iconBytes length:(NSUInteger)iconLength];
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		if (lunaboxStatusItem != nil) {
			lunaboxTrayReady();
			return;
		}

		lunaboxTrayTarget = [[LunaBoxTrayTarget alloc] init];
		lunaboxStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];

		NSButton *button = lunaboxStatusItem.button;
		if (button != nil) {
			if (iconData != nil && [iconData length] > 0) {
				NSImage *image = [[NSImage alloc] initWithData:iconData];
				if (image != nil) {
					[image setSize:NSMakeSize(18, 18)];
					image.template = YES;
					button.image = image;
					button.imagePosition = NSImageOnly;
				} else {
					button.title = @"L";
				}
			} else {
				button.title = @"L";
			}
			button.toolTip = @"LunaBox";
		}

		NSMenu *menu = [[NSMenu alloc] initWithTitle:@"LunaBox"];
		NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"显示主窗口" action:@selector(showMainWindow:) keyEquivalent:@""];
		showItem.target = lunaboxTrayTarget;
		[menu addItem:showItem];
		[menu addItem:[NSMenuItem separatorItem]];
		NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"退出" action:@selector(quitApplication:) keyEquivalent:@""];
		quitItem.target = lunaboxTrayTarget;
		[menu addItem:quitItem];
		lunaboxStatusItem.menu = menu;

		lunaboxTrayReady();
	});
}

void lunaboxTrayStop(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (lunaboxStatusItem != nil) {
			[[NSStatusBar systemStatusBar] removeStatusItem:lunaboxStatusItem];
			lunaboxStatusItem = nil;
		}
		lunaboxTrayTarget = nil;
		lunaboxTrayExit();
	});
}
*/
import "C"
