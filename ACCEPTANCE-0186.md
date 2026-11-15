# Runtime 0.1.86 Chrome Launch Guide

User reported Open Extensions Page starts Chrome's default page. Ordinary Google
Chrome 157.0.8103.78 reproduced this in an isolated headed browser under Xvfb:
external launch with `chrome://extensions/` resulted in `chrome://new-tab-page/`;
`about:blank` resulted in `about:blank`. Probe:
`scripts/test-chrome-launch.cjs`, result `.build/chrome-launch-probe.json`.
No existing user profile, cookies, browser policies or security flags were altered.

Chrome's tagged source `chrome/browser/ui/startup/url_util.cc` filters internal
URLs in external command-line launches. Merely adding --new-window or a slash
does not make the original promise correct. We do not use profile mutation,
debugger attachment or synthetic system keystrokes to bypass that boundary.

The command now says Copy Address And Open Chrome. A clipboard write (with
document-copy fallback) happens before browser launch. The full read-only address
is always visible/selectable and has a separate copy button. Messages tell users
to paste in Chrome's address bar and press Enter; a failed copy is not reported
as success. Backend browser endpoint returns `manualNavigationRequired: true`
and launches a normal blank window, not an internal URL. Windows/Linux launch
arguments and macOS application args use the same internal-target normalization.
Package preparation and API Key handling are unchanged.

Verification includes real Chrome launch probe, Go race/vet, frontend tests,
eight viewport/theme guide checks, successful clipboard write before mocked
browser request, failure fallback to selectable address, and API tests ensuring
browser reopening does not grant credentials or claim internal navigation.
Cross-platform release CI retains native builds, login/upgrade checks and real
MV3-to-native configuration tests. This is not proof of owner-device OS navigation.

Chrome source reference:
https://github.com/chromium/chromium/blob/157.0.8103.78/chrome/browser/ui/startup/url_util.cc
