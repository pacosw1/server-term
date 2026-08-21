import XCTest

/// ScreenshotTests drives the app in the simulator and keeps one picture of
/// every screen. It is a look test for a person, so it asserts only that
/// each screen appears.
final class ScreenshotTests: XCTestCase {
    func testCaptureEveryScreen() throws {
        let app = XCUIApplication()
        app.launch()

        // Wait for the first poll to land, so no picture shows an empty card.
        XCTAssertTrue(app.staticTexts["Servers"].waitForExistence(timeout: 10))
        Thread.sleep(forTimeInterval: 8)
        capture(app, name: "01-servers")

        let firstServer = app.scrollViews.buttons.firstMatch
        if firstServer.waitForExistence(timeout: 5) {
            firstServer.tap()
            Thread.sleep(forTimeInterval: 8)
            capture(app, name: "02-server-detail")
            capture(app, name: "03-server-detail-lower", afterScrollingDown: app)
            for _ in 0..<3 { app.swipeUp() }
            Thread.sleep(forTimeInterval: 2)
            capture(app, name: "03b-server-detail-network")
            for _ in 0..<2 { app.swipeUp() }
            Thread.sleep(forTimeInterval: 2)
            capture(app, name: "03c-server-detail-sensors")
            app.navigationBars.buttons.firstMatch.tap()
            Thread.sleep(forTimeInterval: 1)
        }

        app.tabBars.buttons["Runners"].tap()
        Thread.sleep(forTimeInterval: 6)
        capture(app, name: "04-runners")

        app.tabBars.buttons["Agents"].tap()
        Thread.sleep(forTimeInterval: 6)
        capture(app, name: "05-agents")

        let firstAgent = app.scrollViews.buttons.firstMatch
        if firstAgent.waitForExistence(timeout: 3) {
            firstAgent.tap()
            Thread.sleep(forTimeInterval: 3)
            capture(app, name: "06-agent-detail")
            app.navigationBars.buttons.firstMatch.tap()
            Thread.sleep(forTimeInterval: 1)
        }

        app.tabBars.buttons["Settings"].tap()
        Thread.sleep(forTimeInterval: 2)
        capture(app, name: "07-settings")
    }

    private func capture(_ app: XCUIApplication, name: String, afterScrollingDown scroll: XCUIApplication? = nil) {
        if scroll != nil {
            app.swipeUp()
            app.swipeUp()
            Thread.sleep(forTimeInterval: 2)
        }
        let shot = XCTAttachment(screenshot: app.screenshot())
        shot.name = name
        shot.lifetime = .keepAlways
        add(shot)
    }
}
