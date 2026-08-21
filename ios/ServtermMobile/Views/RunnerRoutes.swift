import Foundation
import ServtermKit

/// JobRoute names one job of one server, so a list can push into it.
struct JobRoute: Hashable {
    let server: ServerEntry
    let job: RunnerJob
}

/// RunnerRoute names the runner screen of one server.
struct RunnerRoute: Hashable {
    let server: ServerEntry
}
