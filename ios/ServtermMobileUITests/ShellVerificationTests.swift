import XCTest

/// ShellVerificationTests drives the real app on the real phone, with the
/// key that lives in that phone's Secure Enclave. No other machine can sign
/// with that key, so this is the only way to test the path after
/// authentication.
///
/// Every step is guarded: the test taps only an element of this app, and it
/// stops at once when the app is no longer in front. A blind tap on a
/// personal phone can reach another app, so this test never taps a
/// coordinate and never presses and holds.
final class ShellVerificationTests: XCTestCase {
    private var app: XCUIApplication!

    override func setUp() {
        continueAfterFailure = false
    }

    func testShellOnRealHost() throws {
        app = XCUIApplication()
        app.launch()
        XCTAssertTrue(app.staticTexts["Servers"].waitForExistence(timeout: 20))
        Thread.sleep(forTimeInterval: 6)

        try tap(app.scrollViews.buttons.firstMatch, "the first server card", timeout: 15)
        try tap(app.buttons["open-shell"].firstMatch, "the shell sessions row", timeout: 15)
        wait(20, "the first session list to load")
        capture("v1-session-list")

        // The sheet offers the default name, so this test types nothing
        // here: a text field that is not there would send taps astray.
        try tap(app.buttons["New session"].firstMatch, "the new session button", timeout: 15)
        try tap(app.buttons["Create"].firstMatch, "the create button", timeout: 15)
        wait(25, "the session to appear")
        capture("v2-after-create")

        try tap(app.buttons["session-row"].firstMatch, "the session row", timeout: 20)
        wait(25, "the shell to attach")
        capture("v3-attached")

        try requireForeground("after attaching")
        try type("tput cols; tput lines; echo PTY-OK\n", "the size question")
        wait(6, "the size answer")
        capture("v4-size-before")

        // A window change must move the numbers.
        XCUIDevice.shared.orientation = .landscapeLeft
        wait(6, "the rotation")
        try type("tput cols; tput lines; echo RESIZE-OK\n", "the size question after the rotation")
        wait(6, "the size answer")
        capture("v5-size-after-rotation")
        XCUIDevice.shared.orientation = .portrait
        wait(5, "the rotation back")

        // Control C from the key row must stop a running command.
        try type("sleep 30\n", "a command to interrupt")
        wait(3, "the command to start")
        try tap(app.buttons["ctrl"].firstMatch, "the control key", timeout: 10)
        try type("c", "the control c letter")
        wait(3, "the interrupt")
        try type("echo CTRL-C-RETURNED\n", "the prompt check")
        wait(4, "the prompt")
        capture("v6-control-c")

        // Persistence: start a loop, leave the app, come back.
        try type("(while true; do date >> /tmp/servterm-verify.log; sleep 1; done) &\n", "the loop")
        wait(4, "the loop to start")
        try type("echo LOOP-STARTED\n", "the loop marker")
        wait(3, "the marker")
        capture("v7-loop-started")

        XCUIDevice.shared.press(.home)
        wait(25, "the app to sit in the background")
        app.activate()
        wait(8, "the app to come back")
        capture("v8-after-background")

        try requireForeground("after the background")
        if app.buttons["session-row"].firstMatch.waitForExistence(timeout: 10) {
            try tap(app.buttons["session-row"].firstMatch, "the session row again", timeout: 10)
            wait(25, "the reattach")
        }
        capture("v9-reattached")
        try type("jobs; wc -l < /tmp/servterm-verify.log; echo REATTACH-OK\n", "the proof of the loop")
        wait(6, "the answer")
        capture("v10-proof")

        // Clean up everything this test made on the host.
        try type("kill %1 2>/dev/null; rm -f /tmp/servterm-verify.log; echo CLEANED\n", "the cleanup")
        wait(5, "the cleanup")
        capture("v11-cleaned")
    }

    // MARK: - guarded helpers

    private func tap(_ element: XCUIElement, _ name: String, timeout: TimeInterval) throws {
        try requireForeground("before tapping " + name)
        guard element.waitForExistence(timeout: timeout) else {
            capture("fail-missing-" + name.replacingOccurrences(of: " ", with: "-"))
            XCTFail("\(name) never appeared")
            throw XCTSkip("stop: \(name) never appeared")
        }
        element.tap()
    }

    private func type(_ text: String, _ what: String) throws {
        try requireForeground("before typing " + what)
        guard app.keyboards.count > 0 || app.otherElements.count > 0 else {
            XCTFail("no place to type \(what)")
            throw XCTSkip("stop: no place to type")
        }
        app.typeText(text)
    }

    /// requireForeground stops the test the moment the app is not in front.
    /// This phone belongs to a person, and a stray tap must never reach
    /// another app.
    private func requireForeground(_ when: String) throws {
        guard app.state == .runningForeground else {
            XCTFail("the app is not in front \(when); the test stops here")
            throw XCTSkip("stop: the app is not in front \(when)")
        }
    }

    private func wait(_ seconds: TimeInterval, _ what: String) {
        Thread.sleep(forTimeInterval: seconds)
    }

    private func capture(_ name: String) {
        let shot = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
        shot.name = name
        shot.lifetime = .keepAlways
        add(shot)
    }
}
