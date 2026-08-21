import Foundation
import Testing
@testable import ServtermKit

@Suite("ConfigImport")
struct ConfigTests {
    @Test("the YAML import reads the servers and the orchestrator widget")
    func yaml() throws {
        let imported = try ConfigImport.parse(FixtureJSON.configYAML)
        #expect(imported.servers.count == 2)
        #expect(imported.servers[0].name == "hetzner-32cpu")
        #expect(imported.servers[0].agentURL == "http://100.93.34.43:7843")
        #expect(imported.servers[0].location == "Hetzner EU")
        #expect(imported.servers[1].name == "office-nvrd")
        #expect(imported.servers[1].location == "Office Mac Studio")
        let orchestrator = try #require(imported.orchestrator)
        #expect(orchestrator.name == "pitsa-agents")
        #expect(orchestrator.endpoint == "http://100.93.34.43:7844")
    }

    @Test("the import never carries a token file into the app")
    func noTokens() throws {
        let imported = try ConfigImport.parse(FixtureJSON.configYAML)
        #expect(imported.servers.allSatisfy { !$0.agentURL.contains("token") })
    }

    @Test("a server without an agent URL is skipped")
    func skipsServerWithoutAgent() throws {
        let text = """
        servers:
          - name: no-agent
            address: 10.0.0.1
          - name: with-agent
            agent_url: http://10.0.0.2:7843
        """
        let imported = try ConfigImport.parse(text)
        #expect(imported.servers.map(\.name) == ["with-agent"])
    }

    @Test("the JSON import reads the same shape")
    func json() throws {
        let text = """
        {"servers":[{"name":"a","agent_url":"http://10.0.0.1:7843","location":"Lab"}],
         "widgets":[{"name":"w","type":"orchestrator","endpoint":"http://10.0.0.1:7844"}]}
        """
        let imported = try ConfigImport.parse(text)
        #expect(imported.servers.map(\.name) == ["a"])
        #expect(imported.orchestrator?.endpoint == "http://10.0.0.1:7844")
    }

    @Test("a text with no server is an error")
    func emptyText() {
        #expect(throws: ServtermError.self) {
            _ = try ConfigImport.parse("hello: world")
        }
    }

    @Test("the app config keeps the servers in order and finds one by id")
    func appConfig() {
        var config = AppConfig()
        let first = ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843", location: "A")
        let second = ServerEntry(name: "two", agentURL: "http://10.0.0.2:7843", location: "B")
        config.servers = [first, second]
        #expect(config.servers.map(\.name) == ["one", "two"])
        config.remove(serverID: first.id)
        #expect(config.servers.map(\.name) == ["two"])
    }

    @Test("the config survives a save and a load")
    func roundTrip() throws {
        var config = AppConfig()
        config.servers = [ServerEntry(name: "one", agentURL: "http://10.0.0.1:7843", location: "A")]
        config.orchestrator = OrchestratorEntry(name: "agents", endpoint: "http://10.0.0.1:7844")
        let data = try JSONEncoder().encode(config)
        let loaded = try JSONDecoder().decode(AppConfig.self, from: data)
        #expect(loaded.servers[0].id == config.servers[0].id)
        #expect(loaded.orchestrator?.endpoint == "http://10.0.0.1:7844")
    }
}
