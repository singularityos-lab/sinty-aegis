# Contributing to sinty-aegis

Thanks for your interest in contributing!

## Quick Start

```bash
git clone https://github.com/singularityos-lab/sinty-aegis
cd sinty-aegis
go build ./...
go test ./...
```

## Guidelines

- Keep changes additive and tested. sinty-aegis decides what interrupts a
  person in an emergency; prefer small, verifiable commits over large rewrites.
- A source is added only when an authority publishes it. Test parsers against
  a capture of the real feed before changing them.
- Match the surrounding style. Comments explain WHY, not WHAT.
- By submitting a contribution you agree to the [CLA](CLA.md).

## License

GPL-3.0-only.
