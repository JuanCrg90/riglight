import ast
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).parent
BASE_UNITS = {
    "riglight.service": {
        "description": "Riglight dashboard",
        "user": "riglight",
        "environment": "/etc/riglight/riglight.env",
        "executable": "/usr/local/bin/riglight",
        "arguments": "$RIGLIGHT_ARGS",
    },
    "riglight-docker-proxy.service": {
        "description": "Riglight Docker proxy",
        "user": "riglight-docker-proxy",
        "environment": "/etc/riglight/docker-proxy.env",
        "executable": "/usr/local/bin/riglight-docker-proxy",
        "arguments": "$RIGLIGHT_DOCKER_ARGS",
    },
}
HARDENING = {
    "Type": "simple",
    "Restart": "on-failure",
    "RestartSec": "5s",
    "KillSignal": "SIGTERM",
    "KillMode": "control-group",
    "TimeoutStopSec": "10s",
    "UMask": "0077",
    "NoNewPrivileges": "yes",
    "CapabilityBoundingSet": "",
    "AmbientCapabilities": "",
    "ProtectSystem": "strict",
    "ProtectHome": "yes",
    "PrivateTmp": "yes",
    "PrivateDevices": "yes",
    "ProtectKernelTunables": "yes",
    "ProtectKernelModules": "yes",
    "ProtectControlGroups": "yes",
    "RestrictSUIDSGID": "yes",
    "RestrictRealtime": "yes",
    "RestrictNamespaces": "yes",
    "LockPersonality": "yes",
    "RestrictAddressFamilies": "AF_UNIX AF_INET AF_INET6",
    "IPAddressDeny": "any",
    "IPAddressAllow": "localhost",
}
FORBIDDEN_DIRECTIVES = {
    "DynamicUser",
    "SupplementaryGroups",
    "PrivateNetwork",
    "ProcSubset",
    "ProtectProc",
    "RootDirectory",
    "RootImage",
    "ReadWritePaths",
    "BindPaths",
    "BindReadOnlyPaths",
    "DeviceAllow",
    "DevicePolicy",
}


def parse_unit(path):
    sections = {}
    section = None
    for number, raw in enumerate(path.read_text().splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("[") and line.endswith("]"):
            section = line[1:-1]
            if section in sections:
                raise AssertionError(f"{path}:{number}: repeated section {section}")
            sections[section] = {}
            continue
        if section is None or "=" not in line:
            raise AssertionError(f"{path}:{number}: invalid unit syntax")
        key, value = line.split("=", 1)
        if key in sections[section]:
            raise AssertionError(f"{path}:{number}: repeated directive {key}")
        sections[section][key] = value
    return sections


def parse_single_assignment(path, name):
    assignments = []
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            raise AssertionError(f"{path}: invalid environment-file syntax")
        key, value = line.split("=", 1)
        assignments.append((key, value.strip().strip('"')))
    if len(assignments) != 1 or assignments[0][0] != name:
        raise AssertionError(f"{path}: expected one {name} assignment")
    return assignments[0][1]


class BaseExampleTests(unittest.TestCase):
    def test_base_units_lock_down_identity_lifecycle_and_sandbox(self):
        for filename, expected in BASE_UNITS.items():
            with self.subTest(unit=filename):
                sections = parse_unit(ROOT / filename)
                self.assertEqual(set(sections), {"Unit", "Service", "Install"})
                self.assertEqual(
                    sections["Unit"],
                    {
                        "Description": expected["description"],
                        "After": "network.target",
                        "StartLimitIntervalSec": "60s",
                        "StartLimitBurst": "3",
                    },
                )
                service = sections["Service"]
                self.assertEqual(service["User"], expected["user"])
                self.assertEqual(service["Group"], expected["user"])
                self.assertEqual(service["EnvironmentFile"], expected["environment"])
                self.assertEqual(
                    service["ExecStart"], f"{expected['executable']} {expected['arguments']}"
                )
                self.assertEqual({key: service[key] for key in HARDENING}, HARDENING)
                self.assertTrue(FORBIDDEN_DIRECTIVES.isdisjoint(service))
                self.assertEqual(sections["Install"], {"WantedBy": "multi-user.target"})
                contents = (ROOT / filename).read_text()
                self.assertNotRegex(contents, r"(?im)^\s*Exec(?:Start|Stop|Reload|Condition|StartPre|StartPost|StopPost)=.*\b(?:sh|bash|env)\b")
                self.assertNotRegex(contents, r"(?im)^\s*(?:Wants|Requires|BindsTo|PartOf)=.*(?:docker|tailscale)")
                self.assertNotRegex(contents, r"(?im)^\s*(?:User|Group)=root$")

    def test_environment_examples_have_one_safe_argument_assignment(self):
        dashboard = parse_single_assignment(ROOT / "riglight.env.example", "RIGLIGHT_ARGS")
        docker = parse_single_assignment(
            ROOT / "docker-proxy.env.example", "RIGLIGHT_DOCKER_ARGS"
        )
        self.assertEqual(dashboard, "--listen 127.0.0.1:9090")
        self.assertEqual(
            docker,
            "--listen 127.0.0.1:9123 --socket /run/riglight/docker.sock --container example=example",
        )
        for value in (dashboard, docker):
            self.assertNotRegex(value, r"[$`]")
            self.assertNotRegex(value, r"[;&|]")
        self.assertNotRegex(dashboard, r"--(?:gpu|docker|llama-swap|tailnet|demo|dev)\b")
        self.assertNotIn("0.0.0.0", dashboard)
        self.assertNotIn("0.0.0.0", docker)

    def test_offline_systemd_analyze_verify_substitutes_only_binaries(self):
        analyzer = shutil.which("systemd-analyze")
        self.assertIsNotNone(analyzer, "systemd-analyze is required for offline verification")
        with tempfile.TemporaryDirectory() as temporary:
            stage = Path(temporary)
            staged_units = []
            for filename, expected in BASE_UNITS.items():
                source = (ROOT / filename).read_text()
                original_start = f"ExecStart={expected['executable']} {expected['arguments']}"
                replacement_start = f"ExecStart=/usr/bin/true {expected['arguments']}"
                self.assertEqual(source.count(original_start), 1)
                staged = source.replace(original_start, replacement_start)
                self.assertNotIn(expected["executable"], staged)
                self.assertEqual(staged.count("/usr/bin/true"), 1)
                target = stage / filename
                target.write_text(staged)
                staged_units.append(target)
            result = subprocess.run(
                [analyzer, "verify", *map(str, staged_units)],
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(result.stdout + result.stderr, "")

    def test_harness_uses_no_shell_or_service_manager_control(self):
        tree = ast.parse(Path(__file__).read_text())
        names = {node.id for node in ast.walk(tree) if isinstance(node, ast.Name)}
        self.assertNotIn("systemctl", names)
        self.assertNotIn("systemd_run", names)
        forbidden_commands = {"systemctl", "systemd-run", "useradd", "groupadd", "chmod", "chown"}
        for call in (node for node in ast.walk(tree) if isinstance(node, ast.Call)):
            if isinstance(call.func, ast.Attribute) and call.func.attr == "run":
                command_words = {
                    node.value
                    for node in ast.walk(call.args[0])
                    if isinstance(node, ast.Constant) and isinstance(node.value, str)
                }
                self.assertTrue(forbidden_commands.isdisjoint(command_words))
                self.assertFalse(
                    any(
                        keyword.arg == "shell" and isinstance(keyword.value, ast.Constant) and keyword.value.value
                        for keyword in call.keywords
                    )
                )


if __name__ == "__main__":
    unittest.main()
