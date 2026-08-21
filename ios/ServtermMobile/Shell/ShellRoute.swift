import Foundation
import ServtermKit

/// ShellRoute names the shell screen of one session on one server.
struct ShellRoute: Hashable {
    let server: ServerEntry
    let session: String
}

/// SessionsRoute names the session list of one server.
struct SessionsRoute: Hashable {
    let server: ServerEntry
}
