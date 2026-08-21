import SwiftUI
import ServtermKit
import ServtermSSH

/// SessionsView lists the tmux sessions of one server. A tap attaches to
/// that session. Making a session and killing one are the only writes, and
/// killing asks first.
struct SessionsView: View {
    @Environment(AppModel.self) private var model
    @State private var sessions: SessionsModel = ShellFactory.makeSessions()
    @State private var showsNew = false
    @State private var killTarget: TmuxSession?
    let server: ServerEntry

    var body: some View {
        ZStack {
            PageBackground()
            ScrollView {
                LazyVStack(spacing: Theme.cardSpacing) {
                    if let error = sessions.actionError {
                        ErrorBanner(message: error)
                    }
                    switch sessions.listing {
                    case .loading:
                        Text("Reading the sessions on \(server.host)…")
                            .font(.footnote)
                            .foregroundStyle(Theme.muted)
                            .card()
                    case .failed(let reason):
                        VStack(alignment: .leading, spacing: 8) {
                            Text("The app cannot read the sessions").font(.headline)
                            Text(reason)
                                .font(.footnote)
                                .foregroundStyle(Theme.muted)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        .card()
                    case .sessions(let list) where list.isEmpty:
                        VStack(alignment: .leading, spacing: 8) {
                            Text("No session yet").font(.headline)
                            Text("This host runs no tmux session now. Make one, and it keeps running after you close the app.")
                                .font(.footnote)
                                .foregroundStyle(Theme.muted)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        .card()
                    case .sessions(let list):
                        ForEach(list) { session in
                            NavigationLink(value: ShellRoute(server: server, session: session.name)) {
                                SessionRow(session: session, now: Date())
                            }
                            .buttonStyle(.plain)
                            .accessibilityIdentifier("session-row")
                            .contextMenu {
                                Button("Kill \(session.name)", systemImage: "trash", role: .destructive) {
                                    killTarget = session
                                }
                            }
                        }
                    }
                    Button {
                        showsNew = true
                    } label: {
                        Label("New session", systemImage: "plus")
                            .font(.subheadline)
                            .foregroundStyle(Theme.accent)
                            .frame(maxWidth: .infinity, minHeight: Theme.minimumTapTarget, alignment: .leading)
                            .card()
                    }
                    .buttonStyle(.plain)
                    Text("Hold a session to kill it. Every other action only reads.")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                }
                .padding(.horizontal)
                .padding(.bottom, Theme.cardSpacing)
            }
            .refreshable { await sessions.refresh(server: server) }
        }
        .navigationTitle("Sessions")
        .navigationBarTitleDisplayMode(.inline)
        .task { await sessions.refresh(server: server) }
        .sheet(isPresented: $showsNew) {
            NewSessionSheet { name in
                Task { await sessions.create(name: name, server: server) }
            }
        }
        .confirmationDialog(
            "Kill the session \(killTarget?.name ?? "")?",
            isPresented: Binding(get: { killTarget != nil }, set: { if !$0 { killTarget = nil } }),
            titleVisibility: .visible
        ) {
            if let target = killTarget {
                Button("Kill \(target.name)", role: .destructive) {
                    Task { await sessions.kill(session: target, server: server) }
                    killTarget = nil
                }
            }
            Button("Keep it", role: .cancel) { killTarget = nil }
        } message: {
            Text("Everything running in that session stops. This cannot be undone.")
        }
    }
}

/// SessionRow is one tmux session.
struct SessionRow: View {
    let session: TmuxSession
    let now: Date

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            VStack(alignment: .leading, spacing: 6) {
                HStack(alignment: .firstTextBaseline) {
                    Text(session.name)
                        .font(.headline)
                        .lineLimit(1)
                    Spacer(minLength: 8)
                    StateChip(
                        text: session.isAttached ? "attached" : "detached",
                        color: session.isAttached ? Theme.normal : Theme.muted,
                        systemImage: session.isAttached ? "bolt.fill" : "moon.zzz")
                }
                HStack(spacing: 12) {
                    Label("^[\(session.windows) window](inflect: true)", systemImage: "square.on.square")
                    Label(Format.duration(seconds: session.idleSeconds(now: now)) + " idle", systemImage: "clock")
                }
                .font(.caption)
                .monospacedDigit()
                .foregroundStyle(Theme.muted)
                Text("started " + session.created.formatted(date: .abbreviated, time: .shortened))
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }
            Image(systemName: "chevron.right")
                .font(.footnote)
                .foregroundStyle(Theme.muted)
                .padding(.top, 4)
        }
        .card()
        .accessibilityElement(children: .combine)
        .accessibilityHint("attaches to this session")
    }
}

/// NewSessionSheet takes a name and checks it before tmux sees it.
struct NewSessionSheet: View {
    @Environment(\.dismiss) private var dismiss
    @State private var name = TmuxCommand.defaultSession
    let onCreate: (String) -> Void

    private var isValid: Bool { TmuxSessionName.isValid(name) }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Session name", text: $name)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    if !isValid {
                        Text(TmuxSessionName.rule)
                            .font(.caption)
                            .foregroundStyle(Theme.critical)
                    }
                } header: {
                    Text("New session").foregroundStyle(Theme.muted)
                } footer: {
                    Text("The session keeps running on the host after you close the app.")
                        .foregroundStyle(Theme.muted)
                }
            }
            .scrollContentBackground(.hidden)
            .background(Theme.base)
            .navigationTitle("New session")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Create") {
                        onCreate(name)
                        dismiss()
                    }
                    .disabled(!isValid)
                }
            }
        }
    }
}
