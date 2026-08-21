import Foundation

/// Real payloads that the servterm agent and the orchestrator daemon return.
/// The tokens and the private data are removed.
enum FixtureJSON {
    static let agentStatus = """
    {"latest_at":"2026-08-21T06:10:57.974178941+02:00","node_id":"node-a","service":"servterm-agent","version":1}
    """

    static let history = """
    [{"version":1,"node_id":"node-a","sample":{
      "At":"2026-08-21T06:10:55.998458518+02:00","Online":true,"Error":"","Latency":153404704,
      "Hostname":"node-a","OS":"Debian GNU/Linux 13 (trixie)","Kernel":"6.12.94+deb13-amd64",
      "UptimeSeconds":348948.74,"CPUPercent":0.7191994996873046,"Cores":32,
      "Load1":0.79,"Load5":4.58,"Load15":3.47,
      "MemTotal":67196661760,"MemAvailable":62230171648,"SwapTotal":17162039296,"SwapFree":16293031936,
      "NetRxRate":11.98,"NetTxRate":13.98,"NetworkInterface":"enp6s0","NetworkType":"ethernet","NetworkLinkMbps":1000,
      "PowerWatts":0,"PowerKnown":false,"BatteryPercent":0,"BatteryKnown":false,"BatteryCharging":false,
      "Disks":[{"Mount":"/run","Device":"tmpfs","FSType":"tmpfs","Total":6719668224,"Used":3072000},
               {"Mount":"/","Device":"/dev/md3","FSType":"ext4","Total":105021104128,"Used":22094196736}],
      "Accelerators":[{"Kind":"GPU","Name":"Intel integrated GPU","Utilization":0,"UtilizationKnown":true}],
      "Processes":[{"PID":513039,"User":"servterm","Command":"ps","CPU":100,"Memory":0,"RSS":4902912}]
    }},
    {"version":1,"node_id":"node-a","sample":{
      "At":"2026-08-21T06:10:56.998458518+02:00","Online":true,"Error":"","Latency":153404704,
      "Hostname":"node-a","OS":"Debian GNU/Linux 13 (trixie)","Kernel":"6.12.94+deb13-amd64",
      "UptimeSeconds":348949.74,"CPUPercent":1.5,"Cores":32,
      "Load1":0.79,"Load5":4.58,"Load15":3.47,
      "MemTotal":67196661760,"MemAvailable":62230171648,"SwapTotal":17162039296,"SwapFree":16293031936,
      "NetRxRate":11.98,"NetTxRate":13.98,"NetworkInterface":"enp6s0","NetworkType":"ethernet","NetworkLinkMbps":1000,
      "PowerWatts":0,"PowerKnown":false,"BatteryPercent":0,"BatteryKnown":false,"BatteryCharging":false,
      "Disks":[],"Accelerators":[],"Processes":[]
    }}]
    """

    static let orchestratorStatus = """
    {"schema_version":1,"at":"2026-08-21T04:11:03.161Z","healthy":true,"mode":"fast",
     "repo":"example/repo",
     "daemon":{"pid":374329,"cpu_percent":0.3,"rss_bytes":50524160,"uptime_seconds":791},
     "budget":{"hour_usd":7.1493,"day_usd":7.1493,"week_usd":7.1493,"hour_limit_usd":5,
               "day_limit_usd":7.5,"week_limit_usd":50,"day_remaining_usd":0.3507,
               "pace_note":"spending is within the pace"},
     "totals":{"input_tokens":10,"output_tokens":20,"cost_usd":0,"live":1,"done":0,"blocked":1},
     "auth":{"mode":"subscription","plan_type":"pro","billed":false},
     "cost_is_estimate":true,
     "disk":{"total_bytes":105021104128,"used_bytes":22094196736,"free_bytes":77544828928},
     "limits":{"weekly":{"used_percent":84,"resets_at":1787331928},"five_hour":null,"plan_type":"pro"},
     "agents":[{"issue":91,"title":"fix the parser","state":"working","cycle":2,"pr_number":null,
                "branch":"agent/91","elapsed_seconds":120,"input_tokens":5,"output_tokens":6,
                "cost_usd":0.5,"pid":1234,"cpu_percent":12.5,"rss_bytes":100,"last_error":"",
                "weekly_percent_used":null,"last_activity":null,"activity_age_seconds":null,
                "turns":3,"children":null,"children_running":0,"children_done":0,"children_failed":0,
                "worktree":"","worktree_disk_bytes":null,"tasks":null}],
     "recent":[{"issue":82,"title":null,"state":"blocked","pr_number":null,"cost_usd":0,
                "last_error":"the worker finished with no pull request."}]}
    """

    /// The shape of the real servterm config.yaml, without any token value.
    static let configYAML = """
    refresh_interval: 3s
    history_size: 60
    servers:
      - name: server-a
        address: 100.64.0.1
        user: root
        location: Site A
        tags: [production, ci, tailscale]
        agent_url: http://100.64.0.1:7843
        token_file: ~/tokens/server-a
      - name: server-b
        address: 100.64.0.2
        user: paco
        location: Site B
        agent_url: http://100.64.0.2:7843
    widgets:
      - name: agents
        type: orchestrator
        endpoint: http://100.64.0.1:7844
        token_file: ~/tokens/agents
      - name: office-nvr
        type: nvr
        endpoint: http://100.64.0.2:8085
    """
}
