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

        // Push into the runner of the first server, and then into a job.
        let runnerHeader = app.buttons["runner-header"].firstMatch
        if runnerHeader.waitForExistence(timeout: 5) {
            runnerHeader.tap()
            Thread.sleep(forTimeInterval: 6)
            capture(app, name: "04b-runner-detail")
            let jobRow = app.buttons["job-row"].firstMatch
            if jobRow.waitForExistence(timeout: 5) {
                jobRow.tap()
                Thread.sleep(forTimeInterval: 12)
                capture(app, name: "04c-job-detail")
                goBack(app)
            }
            goBack(app)
        }

        app.tabBars.buttons["Agents"].tap()
        Thread.sleep(forTimeInterval: 6)
        capture(app, name: "05-agents")

        let firstAgent = app.buttons["agent-row"].firstMatch
        if firstAgent.waitForExistence(timeout: 4) {
            firstAgent.tap()
            Thread.sleep(forTimeInterval: 4)
            capture(app, name: "06-agent-detail")
            app.swipeUp()
            Thread.sleep(forTimeInterval: 1)
            capture(app, name: "06b-agent-detail-lower")
            let tasks = app.buttons["task-summary"].firstMatch
            if tasks.exists {
                tasks.tap()
                Thread.sleep(forTimeInterval: 2)
                capture(app, name: "06c-agent-tasks")
                goBack(app)
            }
            goBack(app)
        }

        app.tabBars.buttons["Settings"].tap()
        Thread.sleep(forTimeInterval: 2)
        capture(app, name: "07-settings")
    }

    /// goBack leaves one screen, if a back button is there to press.
    private func goBack(_ app: XCUIApplication) {
        let back = app.navigationBars.buttons.firstMatch
        if back.exists { back.tap() }
        Thread.sleep(forTimeInterval: 1.5)
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
