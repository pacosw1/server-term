import Foundation
@testable import ServtermKit

/// A test double for the network. It records each request and returns a
/// prepared answer. No test touches the real network.
actor FakeHTTPClient: HTTPClient {
    struct Answer {
        var status: Int = 200
        var body: Data = Data()
        var failure: (any Error)?
    }

    private var answers: [Answer]
    private(set) var requests: [URLRequest] = []

    init(answers: [Answer]) {
        self.answers = answers
    }

    init(status: Int = 200, body: String = "", failure: (any Error)? = nil) {
        self.answers = [Answer(status: status, body: Data(body.utf8), failure: failure)]
    }

    func send(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        requests.append(request)
        let answer = answers.count > 1 ? answers.removeFirst() : (answers.first ?? Answer())
        if let failure = answer.failure { throw failure }
        let response = HTTPURLResponse(
            url: request.url!, statusCode: answer.status, httpVersion: nil, headerFields: nil)!
        return (answer.body, response)
    }
}
