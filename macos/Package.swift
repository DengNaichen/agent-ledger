// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "InfTaskPreview",
    platforms: [.macOS(.v14)],
    targets: [
        .executableTarget(name: "InfTaskPreview", resources: [.copy("Resources")])
    ]
)
