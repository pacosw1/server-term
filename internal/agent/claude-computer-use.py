import os, json, tempfile, fcntl
home = os.path.expanduser("~")
config = os.path.join(home, ".claude.json")
with open(os.path.join(home, ".servterm-claude-config.lock"), "a") as lock:
    fcntl.flock(lock, fcntl.LOCK_EX)
    original = open(config, "rb").read() if os.path.exists(config) else None
    data = json.loads(original) if original is not None else {}
    project = data.setdefault("projects", {}).setdefault(os.path.realpath(os.getcwd()), {})
    enabled = project.setdefault("enabledMcpServers", [])
    if "computer-use" not in enabled:
        enabled.append("computer-use")
    disabled = project.get("disabledMcpServers", [])
    if "computer-use" in disabled:
        disabled.remove("computer-use")
    fd, temporary = tempfile.mkstemp(prefix=".servterm-claude-", dir=home)
    try:
        with os.fdopen(fd, "w") as output:
            json.dump(data, output, indent=2)
            output.write("\n")
        current = open(config, "rb").read() if os.path.exists(config) else None
        if current != original:
            raise RuntimeError("Claude settings changed during startup. Try again.")
        os.replace(temporary, config)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
