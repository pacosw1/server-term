import SwiftUI
import ServtermKit

struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @State private var editedServer: ServerEntry?
    @State private var addsServer = false
    @State private var editsOrchestrator = false
    @State private var showsImport = false
    @State private var showsDetect = false

    var body: some View {
        NavigationStack {
            List {
                Section("Servers") {
                    ForEach(model.config.servers) { server in
                        Button {
                            editedServer = server
                        } label: {
                            VStack(alignment: .leading, spacing: 2) {
                                Text(server.name).foregroundStyle(.primary)
                                Text(server.agentURL).font(.caption).foregroundStyle(.secondary)
                            }
                        }
                        .swipeActions {
                            Button("Remove", role: .destructive) { model.remove(server: server) }
                        }
                    }
                    Button("Add a server") { addsServer = true }
                }

                Section("Orchestrator") {
                    if let entry = model.config.orchestrator {
                        Button {
                            editsOrchestrator = true
                        } label: {
                            VStack(alignment: .leading, spacing: 2) {
                                Text(entry.name).foregroundStyle(.primary)
                                Text(entry.endpoint).font(.caption).foregroundStyle(.secondary)
                            }
                        }
                        Button("Remove the orchestrator", role: .destructive) {
                            model.setOrchestrator(nil, token: nil)
                        }
                    } else {
                        Button("Add the orchestrator") { editsOrchestrator = true }
                    }
                }

                Section("Tools") {
                    Button("Detect the servterm ports") { showsDetect = true }
                    Button("Import a config") { showsImport = true }
                }

                Section("How the app reads your servers") {
                    Text("""
                        The app reads the servterm agent over your tailnet. \
                        Your phone must run Tailscale and must be in the same tailnet. \
                        The app cannot ask the Tailscale control plane for your machine list. \
                        Therefore it cannot find a host by itself. \
                        You paste a host or an IP address, and the app tests port 7843 for an agent \
                        and port 7844 for the orchestrator.
                        """)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                    Text("""
                        The app keeps every token in the iOS Keychain. \
                        It keeps the host list in the app settings. \
                        It sends each token only to the address that you gave for it. \
                        The connections use plain HTTP, because the tailnet already protects them.
                        """)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Settings")
            .sheet(isPresented: $addsServer) {
                ServerEditView(server: nil)
            }
            .sheet(item: $editedServer) { server in
                ServerEditView(server: server)
            }
            .sheet(isPresented: $editsOrchestrator) {
                OrchestratorEditView()
            }
            .sheet(isPresented: $showsImport) {
                ImportView()
            }
            .sheet(isPresented: $showsDetect) {
                DetectView()
            }
        }
    }
}

/// ServerEditView adds one server or changes one server.
struct ServerEditView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss

    let server: ServerEntry?
    @State private var name = ""
    @State private var agentURL = "http://"
    @State private var location = ""
    @State private var token = ""

    var body: some View {
        NavigationStack {
            Form {
                Section("Server") {
                    TextField("Name", text: $name)
                    TextField("Agent URL", text: $agentURL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                    TextField("Location", text: $location)
                }
                Section("Token") {
                    SecureField("Bearer token", text: $token)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Text("The app stores the token in the iOS Keychain. It never writes the token to the app settings.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle(server == nil ? "Add a server" : "Edit the server")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(name.isEmpty || agentURL.count < 8)
                }
            }
            .onAppear(perform: load)
        }
    }

    private func load() {
        guard let server else { return }
        name = server.name
        agentURL = server.agentURL
        location = server.location
        token = model.token(for: server.id)
    }

    private func save() {
        let entry = ServerEntry(
            id: server?.id ?? UUID(),
            name: name.trimmingCharacters(in: .whitespaces),
            agentURL: agentURL.trimmingCharacters(in: .whitespaces),
            location: location.trimmingCharacters(in: .whitespaces))
        model.upsert(server: entry, token: token)
        dismiss()
    }
}

/// OrchestratorEditView sets the orchestrator endpoint and its token.
struct OrchestratorEditView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss

    @State private var name = "orchestrator"
    @State private var endpoint = "http://"
    @State private var token = ""

    var body: some View {
        NavigationStack {
            Form {
                Section("Orchestrator") {
                    TextField("Name", text: $name)
                    TextField("Endpoint", text: $endpoint)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                    SecureField("Bearer token", text: $token)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                }
                Section {
                    Text("The app only reads the orchestrator. It never changes the mode and it never steers an agent.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Orchestrator")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") { save() }
                        .disabled(name.isEmpty || endpoint.count < 8)
                }
            }
            .onAppear(perform: load)
        }
    }

    private func load() {
        guard let entry = model.config.orchestrator else { return }
        name = entry.name
        endpoint = entry.endpoint
        token = model.token(for: entry.id)
    }

    private func save() {
        let entry = OrchestratorEntry(
            id: model.config.orchestrator?.id ?? UUID(),
            name: name.trimmingCharacters(in: .whitespaces),
            endpoint: endpoint.trimmingCharacters(in: .whitespaces))
        model.setOrchestrator(entry, token: token)
        dismiss()
    }
}

/// DetectView tests the standard servterm ports on one host.
struct DetectView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss

    @State private var host = ""
    @State private var results: [ProbeResult] = []
    @State private var isBusy = false

    var body: some View {
        NavigationStack {
            Form {
                Section("Host") {
                    TextField("Tailnet host or IP address", text: $host)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button(isBusy ? "Testing…" : "Test the ports") { detect() }
                        .disabled(host.isEmpty || isBusy)
                }
                if !results.isEmpty {
                    Section("Result") {
                        ForEach(results) { result in
                            VStack(alignment: .leading, spacing: 2) {
                                HStack {
                                    Text("\(result.kind.rawValue) · port \(result.port)")
                                    Spacer()
                                    StateBadge(
                                        text: result.reachable ? "answers" : "no answer",
                                        color: result.reachable ? .green : .red)
                                }
                                Text(result.detail).font(.caption).foregroundStyle(.secondary)
                                Text(result.url).font(.caption).foregroundStyle(.secondary)
                            }
                        }
                    }
                }
                Section {
                    Text("""
                        The app tests only the host that you give. \
                        It cannot list the machines of your tailnet. \
                        A port that answers still needs a token, so add the server \
                        or the orchestrator after the test.
                        """)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Detect")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Close") { dismiss() } }
            }
        }
    }

    private func detect() {
        isBusy = true
        Task {
            results = await model.probe(host: host)
            isBusy = false
        }
    }
}

/// ImportView reads a pasted servterm config. It reads the hosts only. It
/// never reads a token, because a token file stays on the desktop machine.
struct ImportView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss

    @State private var text = ""
    @State private var message = ""
    @State private var failed = false

    var body: some View {
        NavigationStack {
            Form {
                Section("Paste the config") {
                    TextEditor(text: $text)
                        .frame(minHeight: 200)
                        .font(.system(.footnote, design: .monospaced))
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                }
                if !message.isEmpty {
                    Text(message).font(.footnote).foregroundStyle(failed ? .red : .green)
                }
                Section {
                    Text("""
                        The app reads the servers and the orchestrator widget from your \
                        servterm config.yaml or from the same shape in JSON. \
                        It does not read a token: the token_file line names a file on your \
                        desktop machine. Add each token by hand after the import.
                        """)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Import")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Close") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Import") { runImport() }.disabled(text.isEmpty)
                }
            }
        }
    }

    private func runImport() {
        do {
            let imported = try ConfigImport.parse(text)
            model.merge(imported)
            failed = false
            message = "The app added \(imported.servers.count) servers. Now add each token."
        } catch let error as ServtermError {
            failed = true
            message = error.message
        } catch {
            failed = true
            message = error.localizedDescription
        }
    }
}
