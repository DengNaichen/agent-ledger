import SwiftUI
import CoreText

@main
struct InfTaskPreviewApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate

    init() {
        let font = Bundle.module.url(forResource: "InterVariable", withExtension: "ttf", subdirectory: "Resources")!
        CTFontManagerRegisterFontsForURL(font as CFURL, .process, nil)
    }

    var body: some Scene {
        Window("InfTask Preview", id: "preview") {
            ShellView()
        }
        .defaultSize(width: 1280, height: 840)
        .windowStyle(.hiddenTitleBar)
        .windowResizability(.contentMinSize)
        .commands {
            CommandGroup(replacing: .newItem) { }
            CommandGroup(replacing: .appInfo) {
                Button("About InfTask Preview") {
                    NSApp.orderFrontStandardAboutPanel(options: [
                        .applicationName: "InfTask Preview",
                        .applicationVersion: "0.1 — Offline visual preview",
                        .credits: NSAttributedString(string: "SwiftUI + AppKit · No WebView\nFictional data only. No network or database access.\nInter (SIL OFL) · Lucide geometry (ISC)")
                    ])
                }
            }
        }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }
}
