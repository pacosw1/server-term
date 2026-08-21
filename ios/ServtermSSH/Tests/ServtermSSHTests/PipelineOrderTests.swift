import Crypto
import Foundation
import NIOCore
import NIOEmbedded
import NIOSSH
import Testing
@testable import ServtermSSH

/// NIOSSH reports a finished authentication with
/// context.fireUserInboundEventTriggered, which walks the pipeline towards
/// the TAIL. A handler placed before the SSH handler therefore never sees
/// it. The app waited for that event, so it waited forever: sshd accepted
/// the key, opened a session, and then nothing was ever asked of it.
@Suite("Pipeline order")
struct PipelineOrderTests {
    /// AuthEventSource stands in for NIOSSHHandler: it reports a finished
    /// authentication the same way, from its own place in the pipeline.
    private final class AuthEventSource: ChannelInboundHandler, @unchecked Sendable {
        typealias InboundIn = Any

        func channelActive(context: ChannelHandlerContext) {
            context.fireUserInboundEventTriggered(UserAuthSuccessEvent())
            context.fireChannelActive()
        }
    }

    private func connect(_ channel: EmbeddedChannel) throws {
        try channel.connect(to: SocketAddress(unixDomainSocketPath: "/tmp/order")).wait()
    }

    @Test("a watcher after the SSH handler sees the authentication")
    func watcherAfterHandler() throws {
        let channel = EmbeddedChannel()
        let watcher = AuthWatcher()
        try channel.pipeline.syncOperations.addHandler(AuthEventSource())
        try channel.pipeline.syncOperations.addHandler(watcher)
        try connect(channel)
        #expect(watcher.hasAuthenticated, "the watcher must see the event of the handler before it")
        _ = try? channel.finish()
    }

    @Test("a watcher before the SSH handler never sees it, which is the old bug")
    func watcherBeforeHandler() throws {
        let channel = EmbeddedChannel()
        let watcher = AuthWatcher()
        try channel.pipeline.syncOperations.addHandler(watcher)
        try channel.pipeline.syncOperations.addHandler(AuthEventSource())
        try connect(channel)
        #expect(
            watcher.hasAuthenticated == false,
            "an event walks towards the tail, so a handler in front cannot see it")
        _ = try? channel.finish()
    }

    @Test("the client builds its pipeline with the SSH handler first")
    func clientPipelineOrder() throws {
        let identity = try SSHIdentity.generateInMemory(comment: "order")
        let watcher = AuthWatcher()
        let handlers = SSHPipeline.clientHandlers(
            host: "10.0.0.1",
            user: "root",
            key: identity.nioKey,
            checker: HostKeyChecker(store: MemoryFingerprintStore()),
            watcher: watcher,
            allocator: ByteBufferAllocator())
        #expect(handlers.count == 2)
        #expect(handlers[0] is NIOSSHHandler, "the SSH handler must come first")
        #expect(handlers[1] === watcher, "the watcher must sit behind it, or it hears nothing")
    }

    @Test("the wait for the authentication finishes once the event arrives")
    func waitCompletes() throws {
        let channel = EmbeddedChannel()
        let watcher = AuthWatcher()
        try channel.pipeline.syncOperations.addHandler(AuthEventSource())
        try channel.pipeline.syncOperations.addHandler(watcher)
        try connect(channel)

        let done = Flag()
        watcher.authentication(on: channel, timeout: .seconds(20)).whenSuccess { done.set() }
        channel.embeddedEventLoop.run()
        #expect(done.isSet)
        _ = try? channel.finish()
    }
}

private final class Flag: @unchecked Sendable {
    private(set) var isSet = false
    func set() { isSet = true }
}
