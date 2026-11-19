set shell := ["bash", "-euo", "pipefail", "-c"]

# List the recipes.
default:
    @just --list

# Build the CLI into bin/revisa.
build:
    go build -o bin/revisa ./cmd/revisa

# Run the Go and Neovim tests.
test:
    go test ./...

# Tag vVERSION from origin/main and publish a GitHub release with NOTES.
release VERSION NOTES:
    #!/usr/bin/env bash
    set -euo pipefail
    version="{{VERSION}}"
    notes="{{NOTES}}"
    tag="v$version"
    if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "release: VERSION must be X.Y.Z, got '$version'" >&2
        exit 1
    fi
    if [[ ! -s "$notes" ]]; then
        echo "release: notes file '$notes' is missing or empty" >&2
        exit 1
    fi
    if [[ "$(git branch --show-current)" != "main" ]]; then
        echo "release: check out main first" >&2
        exit 1
    fi
    if [[ -n "$(git status --porcelain)" ]]; then
        echo "release: the working tree is not clean" >&2
        exit 1
    fi
    git fetch --tags origin
    if [[ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]]; then
        echo "release: main differs from origin/main" >&2
        exit 1
    fi
    if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
        echo "release: tag $tag already exists" >&2
        exit 1
    fi
    just test
    git tag -a "$tag" -m "$tag"
    git push origin "$tag"
    gh release create "$tag" --title "$tag" --notes-file "$notes" --verify-tag
