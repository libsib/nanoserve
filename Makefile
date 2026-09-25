# Usage: make release TAG=v0.2.11
# Tags the latest upstream/main (not the current branch) and pushes the tag.

release:
	@test -n "$(TAG)" || (echo "TAG is required, e.g. make release TAG=v0.2.11" && exit 1)
	git fetch upstream
	git tag -a $(TAG) -m "$(TAG)" upstream/main
	git push origin $(TAG)
	git push upstream $(TAG)
