#!/usr/bin/env sh
# Bump patch/minor/major in VERSION and package.json.
# Usage: scripts/bump-semver.sh [patch|minor|major]
set -eu

root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
level="${1:-patch}"
version_file="$root/VERSION"
package_json="$root/package.json"

if [ ! -f "$version_file" ]; then
  echo "error: missing VERSION file at ${version_file}" >&2
  exit 1
fi

current="$(tr -d '[:space:]' <"$version_file")"
case "$current" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *)
    echo "error: invalid VERSION: ${current} (expected X.Y.Z)" >&2
    exit 1
    ;;
esac

major="${current%%.*}"
rest="${current#*.}"
minor="${rest%%.*}"
patch="${rest#*.}"

case "$level" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
  *)
    echo "error: level must be patch|minor|major" >&2
    exit 1
    ;;
esac

next="${major}.${minor}.${patch}"
printf '%s\n' "$next" >"$version_file"

if [ -f "$package_json" ]; then
  tmp="$(mktemp)"
  sed -E "s/(\"version\"[[:space:]]*:[[:space:]]*\")[^\"]+(\")/\\1${next}\\2/" "$package_json" >"$tmp"
  mv "$tmp" "$package_json"
fi

echo "VERSION=${next}"
echo "Tag with: git tag \"v\$(tr -d '[:space:]' < VERSION)\" && git push origin \"v\$(tr -d '[:space:]' < VERSION)\""
