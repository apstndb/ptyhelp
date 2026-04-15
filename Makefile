.PHONY: readme test

# Refresh embedded CLI help in README.md (dogfood: ptyhelp patch).
# Subcommands print flag.Usage to stderr; merge into stdout so patch captures it.
readme:
	go run . patch -file README.md -marker readme-help -- go run . help
	go run . patch -file README.md -marker readme-run-help -- sh -c 'go run . run --help 2>&1'
	go run . patch -file README.md -marker readme-patch-help -- sh -c 'go run . patch --help 2>&1'

test:
	go test ./...
