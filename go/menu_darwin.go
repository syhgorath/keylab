//go:build webview && darwin

package main

// macOS ana menüsü: gömülü webview kendi başına menü kurmaz; menü olmazsa Cmd+C/V/X/A/Z ve Cmd+Q çalışmaz (terminalde yapıştırma dahil).
// Ana iş parçacığında, webview.New'dan sonra ve Run'dan önce çağrılır.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void keylabInstallMenu(const char *cname) {
	NSString *name = [NSString stringWithUTF8String:cname];
	NSMenu *bar = [[NSMenu alloc] init];

	NSMenuItem *appItem = [[NSMenuItem alloc] init];
	[bar addItem:appItem];
	NSMenu *appMenu = [[NSMenu alloc] initWithTitle:name];
	[appMenu addItemWithTitle:[@"Hide " stringByAppendingString:name] action:@selector(hide:) keyEquivalent:@"h"];
	[appMenu addItem:[NSMenuItem separatorItem]];
	[appMenu addItemWithTitle:[@"Quit " stringByAppendingString:name] action:@selector(terminate:) keyEquivalent:@"q"];
	[appItem setSubmenu:appMenu];

	NSMenuItem *editItem = [[NSMenuItem alloc] init];
	[bar addItem:editItem];
	NSMenu *edit = [[NSMenu alloc] initWithTitle:@"Edit"];
	[edit addItemWithTitle:@"Undo" action:@selector(undo:) keyEquivalent:@"z"];
	NSMenuItem *redo = [edit addItemWithTitle:@"Redo" action:@selector(redo:) keyEquivalent:@"z"];
	[redo setKeyEquivalentModifierMask:NSEventModifierFlagCommand | NSEventModifierFlagShift];
	[edit addItem:[NSMenuItem separatorItem]];
	[edit addItemWithTitle:@"Cut" action:@selector(cut:) keyEquivalent:@"x"];
	[edit addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];
	[edit addItemWithTitle:@"Paste" action:@selector(paste:) keyEquivalent:@"v"];
	[edit addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
	[editItem setSubmenu:edit];

	[NSApp setMainMenu:bar];
}
*/
import "C"

import "unsafe"

func installNativeMenu(title string) {
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.keylabInstallMenu(cs)
}
