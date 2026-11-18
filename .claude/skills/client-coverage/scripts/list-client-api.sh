#!/usr/bin/env bash
# Print the public API of the waroom-client-go version pinned in go.mod, and how the provider uses it.
# go.work points to ../waroom-client-go (a local checkout), so use GOWORK=off to read the pinned version.
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

mod=github.com/topotal/waroom-client-go
version=$(GOWORK=off go list -m -f '{{.Version}}' "$mod")
dir=$(GOWORK=off go mod download -json "$mod@$version" | jq -r .Dir)

echo "# client: $mod@$version"
echo "# source: $dir"
echo

echo "## client fields (Client struct field -> Service type)"
awk '/^type Client struct/{f=1;next} f&&/^}/{f=0} f&&/\*[A-Za-z]+Service/{print $1"\t"$2}' "$dir/waroom/client.go"
echo

echo "## service methods (Service<TAB>Method)"
grep -h '^func (s \*[A-Za-z]*Service) [A-Z]' "$dir"/waroom/*.go \
  | grep -v '_test.go' \
  | sed -E 's/^func \(s \*([A-Za-z]+)\) ([A-Za-z]+)\(.*/\1\t\2/' \
  | sort
echo

echo "## provider: client fields referenced in internal/provider"
grep -ohE '\.client\.[A-Z][A-Za-z]+' internal/provider/*.go | sed 's/^\.client\.//' | sort | uniq -c
echo

echo "## provider: registered resources / data sources"
awk '/func \(p \*WaroomProvider\) (Resources|DataSources)/{f=1;print;next} f&&/^}/{f=0} f&&/New[A-Za-z]+,/{gsub(/[\t ,]/,"");print "  "$0}' internal/provider/provider.go
echo

echo "## provider: TypeName suffixes"
grep -hE 'resp.TypeName = req.ProviderTypeName \+ "' internal/provider/*.go \
  | sed -E 's/.*\+ "([^"]+)".*/\1/' | sort | uniq -c
