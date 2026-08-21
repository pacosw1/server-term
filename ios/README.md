# ServtermMobile

The iPhone app for servterm: read-only monitoring of the servterm agents,
plus an SSH shell into each host.

## Build

```sh
cd ios
xcodegen generate
xcodebuild -scheme ServtermMobile -destination 'generic/platform=iOS' \
  -derivedDataPath build/dd DEVELOPMENT_TEAM=<TEAM_ID> -allowProvisioningUpdates \
  -skipPackagePluginValidation -skipMacroValidation build
```

`-skipPackagePluginValidation -skipMacroValidation` are REQUIRED. SwiftTerm
ships a build tool plug-in, and a command line build fails with
"Validate plug-in SwiftTermBuildInfoPlugin" without them. Xcode.app asks
once in a dialog instead.

## Install on a device

```sh
xcrun devicectl device install app --device <UDID> \
  build/dd/Build/Products/Debug-iphoneos/ServtermMobile.app
xcrun devicectl device process launch --device <UDID> --terminate-existing com.servterm.mobile
```

The device screen must be awake, or both commands fail with "Locked".

## Tests

```sh
cd ios/ServtermKit && swift test     # decoding, formatting, series, config
cd ios/ServtermSSH && swift test     # host keys, tmux, terminal keys, session state
cd ios && xcodebuild test -scheme ServtermMobile \
  -destination 'platform=iOS Simulator,name=iPhone 17 Pro' \
  -skipPackagePluginValidation -skipMacroValidation
```

The `Screenshots` scheme drives every screen and keeps one picture of each
as a test attachment.

## The key of the phone

The app makes an SSH key at its first start. On a device with a Secure
Enclave the private key is generated inside the chip and never exists as
bytes; a simulator falls back to a P-256 key in the Keychain. The app
writes only the PUBLIC line to `Documents/servterm-public-key.txt`, and the
Settings screen shows it with a copy button. Install that line in
`~/.ssh/authorized_keys` on a host by hand. The app never writes to a host.
