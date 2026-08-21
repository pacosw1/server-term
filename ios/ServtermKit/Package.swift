// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "ServtermKit",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(name: "ServtermKit", targets: ["ServtermKit"])
    ],
    targets: [
        .target(name: "ServtermKit"),
        .testTarget(name: "ServtermKitTests", dependencies: ["ServtermKit"]),
    ]
)
