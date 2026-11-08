"""Download a pinned GitHub asset; verify its contents on every route."""
import hashlib
import json
import os
from pathlib import Path
import sys
import urllib.request


class PublicRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        result = super().redirect_request(req, fp, code, msg, headers, newurl)
        result.remove_header("Authorization")
        return result


def download(repository, tag, name, digest, destination):
    target = Path(destination)
    if target.is_file() and hashlib.sha256(target.read_bytes()).hexdigest() == digest:
        return
    opener = urllib.request.build_opener(PublicRedirect())
    headers = {"User-Agent": "ACU-Runtime-build"}
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    api_headers = dict(headers, Accept="application/vnd.github+json")
    if token:
        api_headers["Authorization"] = "Bearer " + token

    def read(url, request_headers):
        request = urllib.request.Request(url, headers=request_headers)
        with opener.open(request, timeout=45) as response:
            return response.read(128 * 1024 * 1024 + 1)

    direct = f"https://github.com/{repository}/releases/download/{tag}/{name}"
    try:
        data = read(direct, headers)
    except OSError:
        # The API uses the same official release asset, via a separate route.
        release = json.loads(read(f"https://api.github.com/repos/{repository}/releases/tags/{tag}", api_headers))
        asset = next(item for item in release["assets"] if item["name"] == name)
        data = read(asset["url"] + "?download=1", dict(api_headers, Accept="application/octet-stream"))
    if len(data) > 128 * 1024 * 1024 or hashlib.sha256(data).hexdigest() != digest:
        raise ValueError("Release asset SHA-256 mismatch; existing file preserved")
    target.parent.mkdir(parents=True, exist_ok=True)
    temporary = target.with_name(target.name + ".download")
    temporary.write_bytes(data)
    temporary.replace(target)


if __name__ == "__main__":
    download(*sys.argv[1:])
