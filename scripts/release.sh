#!/usr/bin/env bash
# Build, notarize, and publish a signed macOS release to GitHub (and optionally R2).
#
# Usage:
#   SIGN_KEYCHAIN_PASS='...' scripts/release.sh 0.1.20
#   UPLOAD_ONLY=1 scripts/release.sh 0.1.20          # skip build/notarize, upload existing DMG
#   PUBLISH_R2=1 scripts/release.sh 0.1.20           # also push to releases.re-mem-ber.me
#
# Credentials (optional): ~/.rmb/release.env
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${1:?usage: release.sh <version>  e.g. 0.1.20}"
UPLOAD_ONLY="${UPLOAD_ONLY:-0}"
PUBLISH_R2="${PUBLISH_R2:-0}"
SKIP_NOTARIZE="${SKIP_NOTARIZE:-0}"

if [[ -f "$HOME/.rmb/release.env" ]]; then
  # shellcheck disable=SC1091
  source "$HOME/.rmb/release.env"
fi

DMG="$ROOT/dist/RMB Desktop_${VERSION}_aarch64.dmg"
DMG_TMP="/tmp/RMB.Desktop_${VERSION}_aarch64.dmg"
NOTES_FILE="/tmp/rmb-release-notes-${VERSION}.md"
REPO="colinleefish/rmb-desktop"
PROXY_URL="${PROXY_URL:-socks5://127.0.0.1:1080}"

gh_cmd() {
  ALL_PROXY="$PROXY_URL" HTTPS_PROXY="$PROXY_URL" HTTP_PROXY="$PROXY_URL" gh "$@"
}

write_release_notes() {
  local prev_tag
  prev_tag="$(git describe --tags --abbrev=0 2>/dev/null || true)"
  {
    echo "# RMB Desktop ${VERSION}"
    echo
    if [[ -n "$prev_tag" ]]; then
      echo "Changes since ${prev_tag}:"
      echo
      git log --pretty=format:'- %s (%h)' "${prev_tag}..HEAD" || true
    else
      git log --pretty=format:'- %s (%h)' -20 || true
    fi
    echo
  } >"$NOTES_FILE"
}

if [[ "$UPLOAD_ONLY" != "1" ]]; then
  if [[ -z "${SIGN_KEYCHAIN_PASS:-}" ]]; then
    echo "release: set SIGN_KEYCHAIN_PASS (or add it to ~/.rmb/release.env)" >&2
    exit 1
  fi
  echo "==> build v${VERSION}"
  make app-build VERSION="$VERSION" SIGN_KEYCHAIN_PASS="$SIGN_KEYCHAIN_PASS"
  if [[ "$SKIP_NOTARIZE" != "1" ]]; then
    echo "==> notarize v${VERSION}"
    make notarize VERSION="$VERSION"
  fi
fi

if [[ ! -f "$DMG" ]]; then
  echo "release: missing DMG: $DMG" >&2
  exit 1
fi

echo "==> verify notarization"
if ! xcrun stapler validate "$DMG" >/dev/null 2>&1; then
  echo "release: DMG is not notarized/stapled. Run: make notarize VERSION=${VERSION}" >&2
  exit 1
fi

cp "$DMG" "$DMG_TMP"
write_release_notes

echo "==> publish GitHub release v${VERSION}"
if gh_cmd release view "v${VERSION}" --repo "$REPO" >/dev/null 2>&1; then
  gh_cmd release upload "v${VERSION}" "$DMG_TMP" --repo "$REPO" --clobber
  echo "  uploaded to existing release"
else
  gh_cmd release create "v${VERSION}" "$DMG_TMP" \
    --repo "$REPO" \
    --title "RMB Desktop ${VERSION}" \
    --notes-file "$NOTES_FILE"
  echo "  created new release"
fi

if [[ "$PUBLISH_R2" == "1" ]]; then
  RMB_WEBSITE="${RMB_WEBSITE:-$ROOT/../rmb-website}"
  if [[ -x "$RMB_WEBSITE/scripts/publish-release.sh" ]]; then
    echo "==> publish R2 via rmb-website"
    "$RMB_WEBSITE/scripts/publish-release.sh" "$VERSION" "$DMG_TMP"
  else
    echo "release: PUBLISH_R2=1 but $RMB_WEBSITE/scripts/publish-release.sh not found" >&2
    exit 1
  fi
fi

echo
echo "Done."
echo "  GitHub: https://github.com/${REPO}/releases/tag/v${VERSION}"
echo "  DMG:    $DMG_TMP"
