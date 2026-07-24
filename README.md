# SwiftSearch

A lightweight command-line file search tool built with Go.

SwiftSearch recursively scans a directory tree, builds an in-memory index of filenames to their paths, persists it to a JSON file, and lets you search it later by exact filename match.

## Features

- **Fast Filename Lookup:** In-memory map for instant exact-match search
- **Persistent Index:** Saves the index to `index.json` so you don't have to re-scan every time
- **Zero Dependencies:** Uses only Go's standard library
- **Single Binary:** One compiled binary, no runtime requirements
- **Cross-Platform:** Works on Windows, macOS, and Linux

## How It Works

1. **Index Phase** — Walk a directory tree with `filepath.Walk` and store `lowercase_filename → full_path` in a Go map
2. **Persist Phase** — Serialize the map to `index.json`
3. **Search Phase** — Load `index.json` into memory, look up the query by exact filename match

## Installation

Requires Go 1.19+.

```bash
git clone https://github.com/Kritagya123611/SwiftSearch.git
cd SwiftSearch
go build -o swiftsearch
```

Optional: add to PATH

```bash
sudo mv swiftsearch /usr/local/bin/
```

## Usage

### Step 1 — Index a directory

```bash
./swiftsearch -index -dir ~/Projects
```

This scans the directory and saves the results to `index.json`.

### Step 2 — Search for a file

```bash
./swiftsearch -search "main.go"
```

This loads the saved index and finds the file by exact name.

## CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-index` | Build/update the file index | `false` |
| `-search <query>` | Search for a file by exact name | `""` |
| `-dir <path>` | Directory to index | `"."` |

## Example

```bash
# Build the tool
go build -o swiftsearch

# Index your current project
./swiftsearch -index -dir .

# Search for a file
./swiftsearch -search "cli.go"
```

## Limitations

- **Exact match only** — searching for `"cli"` will not find `"cli.go"`
- **Filename collisions** — if two files have the same name, only the last one found is stored
- **No content search** — only searches filenames, not file contents
- **No incremental indexing** — re-walks the entire directory tree each time
- **No filtering** — indexes everything including `.git` internals

## Roadmap

- [ ] Partial/fuzzy filename matching
- [ ] File content search
- [ ] Skip hidden directories (`.git`, etc.)
- [ ] Incremental indexing
- [ ] File type filtering
- [ ] Multiple results per search
- [ ] Ignore binary/large files

## License

MIT
