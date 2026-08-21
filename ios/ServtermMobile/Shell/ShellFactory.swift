import Foundation

/// ShellFactory decides which models the shell screens hold. In a release
/// build there is only one answer, because the fake transport does not
/// exist there at all.
@MainActor
enum ShellFactory {
    static var usesFakeShell: Bool {
        #if UITEST_SUPPORT
            return UITestSupport.usesFakeShell
        #else
            return false
        #endif
    }

    static func makeShell() -> ShellModel {
        #if UITEST_SUPPORT
            if UITestSupport.usesFakeShell { return UITestSupport.makeShellModel(session: "") }
        #endif
        return ShellModel()
    }

    static func makeSessions() -> SessionsModel {
        #if UITEST_SUPPORT
            if UITestSupport.usesFakeShell { return UITestSupport.makeSessionsModel() }
        #endif
        return SessionsModel()
    }
}
