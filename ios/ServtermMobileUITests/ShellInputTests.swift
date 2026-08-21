import XCTest

/// ShellInputTests proves the last mile that no unit test can reach: a key
/// press in the real TerminalView, and a tap on the real key row, arriving
/// as bytes at the transport, and bytes from the transport being drawn on
/// the screen.
///
/// It runs in the simulator against a fake transport, so it needs no key on
/// any host and no phone.
final class ShellInputTests: XCTestCase {
    private var app: XCUIApplication!

    override func setUp() {
        continueAfterFailure = false
    }

    func testTypingReachesTheTransport() throws {
        app = XCUIApplication()
        app.launchArguments = ["-uiTestFakeShell"]
        app.launch()
        XCTAssertTrue(app.staticTexts["Servers"].waitForExistence(timeout: 20))
        Thread.sleep(forTimeInterval: 4)

        // Reach the shell the way a person does.
        let server = app.scrollViews.buttons.firstMatch
        XCTAssertTrue(server.waitForExistence(timeout: 10), "no server card")
        server.tap()
        let sessions = app.buttons["open-shell"].firstMatch
        XCTAssertTrue(sessions.waitForExistence(timeout: 10), "no shell entry")
        sessions.tap()
        let row = app.buttons["session-row"].firstMatch
        XCTAssertTrue(row.waitForExistence(timeout: 10), "the fake session never listed")
        row.tap()

        // SwiftTerm's view is a UIScrollView, so it is not an "other"
        // element. Search every kind for the identifier.
        let terminal = app.descendants(matching: .any)["terminal-view"].firstMatch
        if !terminal.waitForExistence(timeout: 10) {
            print("ELEMENT TREE: \(app.debugDescription.prefix(3000))")
            XCTFail("no terminal view")
            return
        }
        Thread.sleep(forTimeInterval: 3)
        attach(name: "i1-attached")

        // SwiftTerm's iOS view marks itself as an accessibility element but
        // never publishes the screen text. The screen mirrors what the
        // terminal itself holds into a label instead, so this reads what
        // SwiftTerm parsed, not what the model carried.
        let mirror = app.staticTexts["fake-screen-mirror"].firstMatch
        XCTAssertTrue(mirror.waitForExistence(timeout: 10), "no screen mirror")
        var drew = false
        for _ in 0..<10 {
            if mirror.label.contains("FAKE-READY") { drew = true; break }
            Thread.sleep(forTimeInterval: 1)
        }
        XCTAssertTrue(drew, "the output of the transport was never drawn. mirror was: \(mirror.label)")

        // 1. The keyboard path.
        terminal.tap()
        Thread.sleep(forTimeInterval: 2)
        app.typeText("ls")
        Thread.sleep(forTimeInterval: 2)
        let afterTyping = log()
        attach(name: "i2-after-typing")
        XCTAssertTrue(
            afterTyping.contains("6c") && afterTyping.contains("73"),
            "typing did not reach the transport. log was: \(afterTyping)")

        // 2. The sticky control from the row, with a letter from the
        //    system keyboard: the transport must receive 0x03, not "c".
        let control = app.buttons["ctrl"].firstMatch
        XCTAssertTrue(control.waitForExistence(timeout: 5), "no control key")
        control.tap()
        Thread.sleep(forTimeInterval: 1)
        app.typeText("c")
        Thread.sleep(forTimeInterval: 2)
        attach(name: "i2b-after-control-c")
        XCTAssertTrue(
            log().hasSuffix("03"),
            "control and c did not arrive as the interrupt. log was: \(log())")

        // 3. An arrow from the key row must arrive as its CSI sequence.
        let up = app.buttons["arrow up"].firstMatch
        XCTAssertTrue(up.waitForExistence(timeout: 5), "no arrow key in the row")
        up.tap()
        Thread.sleep(forTimeInterval: 2)
        XCTAssertTrue(
            log().hasSuffix("1b 5b 41"), "the arrow did not arrive. log was: \(log())")

        // 4. The echo of what the transport received must be drawn too.
        var echoed = false
        for _ in 0..<10 {
            if mirror.label.contains("RX ") { echoed = true; break }
            Thread.sleep(forTimeInterval: 1)
        }
        XCTAssertTrue(echoed, "the echo was never drawn. mirror was: \(mirror.label)")
        attach(name: "i3-final")
        print("LAST MILE MIRROR: \(mirror.label)")
        print("LAST MILE LOG: \(log())")
        print("LAST MILE RESIZES: \(app.staticTexts["fake-resize-log"].firstMatch.label)")
    }

    private func log() -> String {
        app.staticTexts["fake-input-log"].firstMatch.label
    }

    private func attach(name: String) {
        let shot = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
        shot.name = name
        shot.lifetime = .keepAlways
        add(shot)
    }
}
