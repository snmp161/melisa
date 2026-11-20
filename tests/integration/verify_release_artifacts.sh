#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import json
import os

with open("dist/artifacts.json") as file:
    artifacts = json.load(file)
expected = {
    "melisa-linux-amd64": "amd64",
    "melisa-linux-arm64": "arm64",
}
binaries = {
    item["name"]: item
    for item in artifacts
    if item.get("type") == "Binary"
    and item.get("extra", {}).get("Format") == "binary"
}
if set(binaries) != set(expected):
    raise SystemExit(f"unexpected release binaries: {sorted(binaries)}")
for name, arch in expected.items():
    artifact = binaries[name]
    if artifact.get("goos") != "linux" or artifact.get("goarch") != arch:
        raise SystemExit(f"invalid target for {name}: {artifact}")
    path = artifact["path"]
    if not os.path.isfile(path) or not os.access(path, os.X_OK):
        raise SystemExit(f"binary is not executable: {path}")
PY

for package in dist/*.deb; do
    contents="$(dpkg-deb --contents "$package")"
    grep -Eq '/usr/bin/melisa$' <<<"$contents"
    grep -Eq '/usr/share/doc/melisa/LICENSE$' <<<"$contents"
    grep -Eq '/usr/share/doc/melisa/NOTICE$' <<<"$contents"
done
for package in dist/*.rpm; do
    contents="$(rpm -qlp "$package")"
    grep -Eq '^/usr/bin/melisa$' <<<"$contents"
    grep -Eq '^/usr/share/doc/melisa/LICENSE$' <<<"$contents"
    grep -Eq '^/usr/share/doc/melisa/NOTICE$' <<<"$contents"
done

echo "Release artifacts verified."
