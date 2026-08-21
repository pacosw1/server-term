import Foundation
import Testing
@testable import ServtermKit

@Suite("Activity tail")
struct ActivityTailTests {
    private let start = Date(timeIntervalSince1970: 1000)

    @Test("the tail keeps the newest line first")
    func order() {
        var tail = ActivityTail()
        tail.record("first", at: start)
        tail.record("second", at: start.addingTimeInterval(1))
        #expect(tail.entries.map(\.text) == ["second", "first"])
        #expect(tail.entries.first?.at == start.addingTimeInterval(1))
    }

    @Test("the same line twice is one entry, because a poll repeats it")
    func distinct() {
        var tail = ActivityTail()
        tail.record("same", at: start)
        tail.record("same", at: start.addingTimeInterval(1))
        tail.record("same", at: start.addingTimeInterval(2))
        #expect(tail.entries.count == 1)
        // The entry keeps the time it first appeared, so the age is honest.
        #expect(tail.entries[0].at == start)
    }

    @Test("a line that comes back after another line is a new entry")
    func repeatAfterChange() {
        var tail = ActivityTail()
        tail.record("a", at: start)
        tail.record("b", at: start.addingTimeInterval(1))
        tail.record("a", at: start.addingTimeInterval(2))
        #expect(tail.entries.map(\.text) == ["a", "b", "a"])
    }

    @Test("the tail keeps only the last six lines")
    func cap() {
        var tail = ActivityTail()
        for index in 0..<10 {
            tail.record("line \(index)", at: start.addingTimeInterval(Double(index)))
        }
        #expect(tail.entries.count == ActivityTail.limit)
        #expect(tail.entries.map(\.text) == (4...9).reversed().map { "line \($0)" })
    }

    @Test("an empty line and a missing line are both ignored")
    func ignoresEmpty() {
        var tail = ActivityTail()
        tail.record(nil, at: start)
        tail.record("", at: start)
        tail.record("   ", at: start)
        #expect(tail.entries.isEmpty)
    }

    @Test("the tail says whether it holds anything yet")
    func emptyState() {
        var tail = ActivityTail()
        #expect(tail.isEmpty)
        tail.record("work", at: start)
        #expect(tail.isEmpty == false)
    }
}

@Suite("Reported lists")
struct ReportedListTests {
    @Test("nil means the daemon does not report this, which is not none")
    func notReported() {
        let list: [Int]? = nil
        #expect(ReportedList.of(list) == .notReported)
    }

    @Test("an empty list means none, which is a fact")
    func none() {
        #expect(ReportedList.of([Int]()) == .none)
    }

    @Test("a filled list carries its items")
    func items() {
        #expect(ReportedList.of([1, 2]) == .items([1, 2]))
    }

    @Test("each state has its own sentence for the screen")
    func sentences() {
        let notReported = ReportedList.of([Int]?.none)
        let none = ReportedList.of([Int]())
        #expect(notReported.message(for: "subagent").contains("does not report"))
        #expect(none.message(for: "subagent").contains("no subagent"))
        #expect(notReported.message(for: "subagent") != none.message(for: "subagent"))
    }
}

@Suite("Run links")
struct RunLinkTests {
    private func job(server: String, repository: String, runID: String) -> RunnerJob {
        var job = RunnerJob()
        job.serverURL = server
        job.repository = repository
        job.runID = runID
        return job
    }

    @Test("a full job builds the run page link")
    func fullLink() {
        let url = job(server: "https://github.com", repository: "owner/repo", runID: "42").runURL
        #expect(url?.absoluteString == "https://github.com/owner/repo/actions/runs/42")
    }

    @Test("a missing part gives no link at all, never a broken one")
    func missingParts() {
        #expect(job(server: "", repository: "owner/repo", runID: "42").runURL == nil)
        #expect(job(server: "https://github.com", repository: "", runID: "42").runURL == nil)
        #expect(job(server: "https://github.com", repository: "owner/repo", runID: "").runURL == nil)
    }

    @Test("a trailing slash on the server does not double the slash")
    func trailingSlash() {
        let url = job(server: "https://github.com/", repository: "owner/repo", runID: "42").runURL
        #expect(url?.absoluteString == "https://github.com/owner/repo/actions/runs/42")
    }
}
