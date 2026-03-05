#!/bin/bash

if ! command -v gh &> /dev/null; then
    echo "GitHub CLI (gh) is not installed. Please install it to proceed."
    exit 1
fi

latest=$(gh release view --json tagName -q '.tagName' 2>/dev/null)
if [ -z "$latest" ]; then
    echo "No existing releases found."
else
    echo "Latest release: $latest"
fi

read -p "Enter the new version (e.g. 1.0.0): " version
if [ -z "$version" ]; then
    echo "No version provided. Aborting."
    exit 1
fi

# Strip leading 'v' if provided
version="${version#v}"

echo ""
read -p "This will release v$version. Continue? (y/N): " confirm
if [ "$confirm" != "y" ]; then
    echo "Aborted."
    exit 1
fi

git tag "v$version" -m "Release version $version"
git push origin "v$version"

gh release create "v$version" --generate-notes

GOPROXY=proxy.golang.org go list -m "github.com/gaucho-racing/ulid-go@v$version"

echo "Package released successfully for version v$version"
