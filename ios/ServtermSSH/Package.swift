// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "ServtermSSH",
    platforms: [.iOS(.v18), .macOS(.v14)],
    products: [
        .library(name: "ServtermSSH", targets: ["ServtermSSH"])
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-nio-ssh.git", from: "0.15.0"),
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.101.0"),
    ],
    targets: [
        .target(
            name: "ServtermSSH",
            dependencies: [
                .product(name: "NIOSSH", package: "swift-nio-ssh"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "NIOPosix", package: "swift-nio"),
            ]),
        .testTarget(
            name: "ServtermSSHTests",
            dependencies: [
                "ServtermSSH",
                .product(name: "NIOEmbedded", package: "swift-nio"),
                .product(name: "NIOCore", package: "swift-nio"),
                .product(name: "NIOSSH", package: "swift-nio-ssh"),
            ]),
    ]
)
