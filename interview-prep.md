# SwiftSearch — Complete Interview Preparation

## Project Understanding

**SwiftSearch** is a CLI file search tool written in Go. It indexes a directory tree into an in-memory map (`filename -> path`) and allows instant lookup via an exact-match query. It uses a goroutine + channel pattern to return results.

**Actual lines of logic:** ~80 (excluding blank lines, imports, comments)
**Files:** `main.go` (5), `cli.go` (42), `indexer.go` (74)
**External dependencies:** None (stdlib only)

---

## FILE-BY-FILE DEEP DIVE

### `main.go` — Entry Point

```go
func main() {
    HandleCLI()
}
```

- **What it does:** Delegates everything to `HandleCLI()`.
- **Why:** Separation of concern — `main` is just an entry point.
- **Alternative:** You could inline everything in `main()`, but this is idiomatic Go.
- **Tradeoff:** Adds a function call, but Go inlines at compile time. No performance impact.
- **Concern:** Nothing. This is clean.

---

### `cli.go` — Command Line Interface

#### Line 9-11: Flag Definitions
```go
indexCmd := flag.Bool("index", false, "Index directory")
searchCmd := flag.String("search", "", "Search query")
dirCmd := flag.String("dir", ".", "Directory to index")
```

- **What it does:** Defines three CLI flags: `-index` (bool), `-search` (string), `-dir` (string).
- **Why `flag` package:** Go's standard library. No external deps needed.
- **Alternative:** `cobra`, `urfave/cli`, `pflag` (supports `--long-flags`).
- **Tradeoff:** `flag` doesn't support `--` double-dash long flags or subcommands natively. You chose zero dependencies over features.
- **Time complexity:** O(1) — flag parsing is trivial.
- **Security concern:** None — no user input is executed or interpolated into shell commands.

#### Line 13: `flag.Parse()`

- **What it does:** Parses `os.Args[1:]` into the defined flags.
- **Why here:** Must be called before accessing any flag values.
- **Edge case:** If someone passes `-index -search foo`, both branches trigger. But the code uses `if/return` pattern, so only `-index` runs (line 18-24 returns early).
- **Bug risk:** If `-index` and `-search` are both passed, only indexing runs. The user gets no search result. This is a silent failure.

#### Line 16: `LoadIndex()`
- **What it does:** Reads `index.json` from CWD into the global `fileIndex` map.
- **Critical design choice:** It always loads at startup, even if you're only indexing. This means:
  - If you run `-index`, you load the old index first, then overwrite the map with new data, then save.
  - If the old index was from a different directory, that data is loaded and discarded.
  - **This is wasteful but not buggy** for the current implementation.

#### Line 18-24: Index Branch
```go
if *indexCmd {
    IndexFiles(*dirCmd)
    SaveIndex()
    return
}
```

- **What it does:** Walks the directory, populates `fileIndex`, saves to disk.
- **Why `return`:** Prevents fall-through to search branch.
- **Edge case:** If `IndexFiles` fails silently (e.g., permission denied on a subdirectory), the error is swallowed by the `filepath.Walk` callback returning `nil`.

#### Line 26-37: Search Branch — THE CRITICAL SECTION
```go
if *searchCmd != "" {
    go SearchFromIndex(*searchCmd)
    result := <-IndexResultChannel
    fmt.Println("🔹", result)
    return
}
```

- **What it does:** Spawns a goroutine for search, then blocks on channel receive.
- **Why a goroutine?** The README claims "concurrent" and "fan-in/fan-out." But in reality, `SearchFromIndex` does a simple map lookup — spawning a goroutine for this is **completely unnecessary overhead**.
- **Time complexity:** O(1) map lookup. Goroutine creation is O(1) but has ~2-3KB stack overhead.
- **Critical bug:** If `-search` is called without prior `-index`, `fileIndex` is empty (or loaded from a missing file). The search will always return "Not found." There's no error message telling the user the index doesn't exist.
- **Goroutine leak:** If `SearchFromIndex` panics, the channel never sends, and `<-IndexResultChannel` blocks forever (deadlock). The commit history shows you fixed a deadlock bug (`2d1044a`).
- **Why only ONE result?** The channel is unbuffered and the code reads exactly once. If the index had multiple matches (it can't with current implementation — map has unique keys), only the first would be returned.
- **Alternative:** Just call `SearchFromIndex(*searchCmd)` directly (no goroutine needed for a map lookup).

---

### `indexer.go` — Core Indexing Logic

#### Line 12: `var fileIndex = make(map[string]string)`

- **What it does:** Global mutable state — maps lowercase filename to its full path.
- **Why global:** All functions need access. In Go, package-level globals are common for simple CLIs.
- **Critical design flaw:** **Only ONE file per filename is tracked.** If two files have the same name (e.g., `README.md` in different directories), the last one walked wins. This is a **silent data loss bug**.
- **Map type:** `map[string]string` — O(1) average lookup via hash table.
- **Space complexity:** O(n) where n = number of files. Each entry stores filename (avg ~15 bytes) + path (avg ~50 bytes) + map overhead (~100 bytes per entry).
- **Concurrency concern:** The global map has **no mutex**. If multiple goroutines write simultaneously, you get a data race. Currently safe only because indexing is single-threaded.
- **Alternative:** `map[string][]string` to support multiple files with the same name.

#### Line 14: `var IndexResultChannel = make(chan string)`

- **What it does:** Unbuffered channel for passing search results.
- **Why unbuffered:** Forces synchronous handoff — sender blocks until receiver is ready.
- **Critical bug:** This is a **global, non-resettable channel.** After one send, the channel is consumed. If you search twice in the same process, the second search will deadlock because nobody is listening on the channel anymore.
- **Why it exists:** The README and git history suggest you were going for a "fan-in/fan-out" pattern, but the current implementation is a simple map lookup that doesn't need channels at all.
- **Alternative:** Just return the value from the function. No channel needed.

#### Line 16: `const indexFile = "index.json"`

- **What it does:** Hardcoded path for index persistence.
- **Security concern:** Writes to CWD. If the tool is run from a directory the user doesn't have write access to, it fails silently (the error is printed but execution continues).
- **Alternative:** Allow user to specify the index file path via a flag.

#### Line 18-31: `SaveIndex()`
```go
func SaveIndex() {
    data, err := json.MarshalIndent(fileIndex, "", "  ")
    err = os.WriteFile(indexFile, data, 0644)
}
```

- **What it does:** Serializes the map to JSON and writes to disk.
- **`json.MarshalIndent`:** Pretty-prints the JSON. O(n) time, O(n) space.
- **`os.WriteFile` with `0644`:** Owner read/write, group/others read. Standard for config files.
- **Security:** The file is world-readable. If the index contains sensitive file paths, this is a leak.
- **Race condition:** If two instances run simultaneously, they can corrupt `index.json`.
- **Improvement:** Write to a temp file then atomically rename (`os.Rename`).

#### Line 34-53: `LoadIndex()`
```go
func LoadIndex() {
    if _, err := os.Stat(indexFile); os.IsNotExist(err) {
        return
    }
    data, err := os.ReadFile(indexFile)
    err = json.Unmarshal(data, &fileIndex)
}
```

- **What it does:** Reads JSON from disk, deserializes into global map.
- **Error handling:** All errors are printed but execution continues. This is a design choice — graceful degradation.
- **Edge case:** Corrupted JSON → `json.Unmarshal` fails → old empty map is used → search returns nothing. No error propagated to user beyond "Error parsing index."
- **`os.Stat` check:** Prevents `ReadFile` from failing on first run. Good defensive coding.
- **Performance:** For a 46-entry index file, this is instant. For 1M files, JSON deserialization would take ~1-2 seconds.

#### Line 55-63: `IndexFiles(root string)`
```go
func IndexFiles(root string) {
    filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil
        }
        fileIndex[strings.ToLower(info.Name())] = path
        return nil
    })
}
```

- **What it does:** Recursively walks the directory tree. For every file/directory, stores `lowercase_name → full_path` in the map.
- **`filepath.Walk`:** Uses `os.Lstat` for each entry. Depth-first traversal.
- **Time complexity:** O(n) where n = total files + directories.
- **Space complexity:** O(d) where d = max directory depth (call stack).
- **Critical issues:**
  1. **Indexes directories too** (`info.IsDir()` is never checked). So `.git` is indexed.
  2. **No filtering** of binary files, hidden files, or vendor directories.
  3. **Last-write-wins** for duplicate filenames.
  4. **`strings.ToLower`** applied to name only, not path. So searching for `README.md` won't find `readme.md` at a different path... wait, it will, because the key is lowered.
- **Error handling:** Returns `nil` on errors, meaning permission-denied directories are silently skipped. This is correct behavior for a search tool.
- **No concurrency:** Despite the README claiming "parallel goroutines," `filepath.Walk` is synchronous. The "concurrency" is only in the search (goroutine + channel), which is unnecessary.
- **Alternative:** `filepath.WalkDir` (faster, avoids `Lstat` calls). `os.ReadDir` for manual control.

#### Line 65-73: `SearchFromIndex(toFind string)`
```go
func SearchFromIndex(toFind string) {
    toFind = strings.ToLower(toFind)
    if path, exists := fileIndex[toFind]; exists {
        IndexResultChannel <- path
    } else {
        IndexResultChannel <- ""
    }
}
```

- **What it does:** Exact-match lookup in the map. Sends result via channel.
- **Time complexity:** O(1) average, O(n) worst case (hash collision).
- **Critical issues:**
  1. **Exact match only.** Searching for "main" finds `main.go`, but searching for "main." or "ma" finds nothing.
  2. **No partial/fuzzy matching.** No prefix search. No substring search.
  3. **Channel send could block** if nobody is listening (deadlock risk).
  4. **Always sends something** (empty string if not found). The caller always receives.
- **Why `strings.ToLower`:** Case-insensitive search. Standard approach.
- **Alternative:** Return `(string, bool)` instead of using a channel.

---

## README vs. CODE DISCREPANCIES

This is **CRITICAL** for interviews. Your README makes claims the code doesn't support:

| README Claim | Actual Code |
|---|---|
| "Parallel goroutines for scanning" | `filepath.Walk` is synchronous, single-threaded |
| "Fan-in/fan-out pattern" | No fan-in/fan-out exists |
| "Dual-mode matching: filename + content" | Only filename matching exists |
| "Content tokenization / text search" | No file content is ever read |
| "Results stream in real-time" | Single result, no streaming |
| "Sub-second lookup via in-memory inverted index" | Technically true, but it's a flat map, not an inverted index |
| "--name-only flag" | Not implemented |
| "--case-sensitive flag" | Not implemented |
| "--max-results flag" | Not implemented |
| CLI usage `./swiftsearch <directory> "<query>"` | Actual usage: `./swiftsearch -index -dir <dir>` then `./swiftsearch -search <query>` |
| "10-30s initial indexing → instant future queries" | Only true for very large directories |

**An experienced interviewer will catch every one of these.**

---

## SECTION 1 — Project Understanding

**Q1: Explain your project in 2 minutes.**

> SwiftSearch is a CLI tool written in Go that indexes filenames in a directory tree into an in-memory hash map and provides instant exact-match lookup. You run `swiftsearch -index -dir <path>` to build the index, then `swiftsearch -search <query>` to find files by name. The index is persisted as JSON to `index.json` so subsequent searches don't require re-indexing.

**Q2: Walk me through the architecture.**

> Three files: `main.go` (entry point), `cli.go` (flag parsing and command routing), `indexer.go` (indexing, persistence, search). The data flow is: CLI flags → parse → branch to index or search → index walks directory tree and populates a `map[string]string` → saves to JSON. Search reads from map, returns via channel.

**Q3: What problem does this solve?**

> Fast file-by-name lookup in large directory trees. Instead of calling `find` or `ls -R` every time, you build an index once and query it in O(1).

**Q4: Why did you build this?**

> To demonstrate Go concurrency primitives (goroutines, channels), CLI design with the `flag` package, filesystem traversal, and JSON serialization. It's a systems programming showcase.

**Q5: What was the biggest challenge?**

> The deadlock bug. The channel-based search pattern required careful orchestration — if the goroutine panics or the channel isn't read, the program hangs. I fixed it in commit `2d1044a`.

**Q6: What would you redesign?**

> 1. Replace the channel with a direct return value (no concurrency needed for a map lookup).
> 2. Change `map[string]string` to `map[string][]string` to handle duplicate filenames.
> 3. Add content indexing (the README promises it but the code doesn't implement it).
> 4. Add partial/fuzzy matching.
> 5. Use `filepath.WalkDir` instead of `filepath.Walk` for better performance.

**Q7: What's the biggest bug?**

> Duplicate filenames. If `src/utils.go` and `lib/utils.go` both exist, the map only stores the last one walked. The user silently loses search results.

**Q8: What would you improve first?**

> Fix the duplicate filename issue, then add content search to match what the README claims, then add partial matching.

---

## SECTION 2 — Code Walkthrough

**Q9: `flag.Bool("index", false, "Index directory")` — why `Bool` and not `String`?**

> Because `-index` is a toggle, not a value. A bool flag is `-index` (true) or absent (false). A string would require `-index true`.

**Q10: Why `flag.Parse()` before `LoadIndex()`?**

> `flag.Parse()` must be called before accessing any flag values (`*indexCmd`, `*searchCmd`). But `LoadIndex()` doesn't depend on flags — it always loads from CWD. I could move it after the parse, but the order doesn't matter functionally.

**Q11: Why `return` after each branch?**

> To prevent fall-through. Without `return`, after indexing, the code would also execute the search branch or the help text. Go doesn't have switch-case fall-through by default, but these are `if` statements, so explicit `return` is needed.

**Q12: `go SearchFromIndex(*searchCmd)` — why a goroutine?**

> Initially, the design intended streaming results via channels. In the current implementation, the goroutine is unnecessary overhead. A direct function call would be simpler and more efficient.

**Q13: `result := <-IndexResultChannel` — what if the channel is empty?**

> The code blocks until a value is sent. Since `SearchFromIndex` always sends (either the path or empty string), this will always unblock — unless the goroutine panics before sending.

**Q14: `fileIndex[strings.ToLower(info.Name())] = path` — what does `ToLower` do to the key?**

> Normalizes filenames to lowercase so the search is case-insensitive. `README.md` and `readme.md` map to the same key. But the value (path) preserves the original case.

**Q15: Why `return nil` in the Walk callback on error?**

> To continue walking even if a specific file/directory has an error (e.g., permission denied). Returning an error would abort the entire walk.

**Q16: `json.MarshalIndent` vs `json.Marshal` — why indent?**

> Readability. The `index.json` file is human-inspectable. Tradeoff: slightly larger file and slower serialization. For a 46-entry index, this is negligible.

**Q17: `os.WriteFile(indexFile, data, 0644)` — what does `0644` mean?**

> Unix file permissions: owner can read/write (6), group can read (4), others can read (4). This is standard for config files. Not executable.

**Q18: `os.Stat(indexFile)` in `LoadIndex` — why check existence first?**

> To distinguish between "file doesn't exist" (first run) and "file exists but can't be read" (permission error). Different error handling for each case.

---

## SECTION 3 — Backend / Systems Questions

**Q19: Explain your data structure. Why a map and not a tree?**

> A hash map provides O(1) average-case lookup. A tree (e.g., trie or BST) would give O(k) lookup where k is the key length, plus O(n) space with node overhead. For exact-match filename search, a map is optimal. A trie would be better for prefix search.

**Q20: What's the time complexity of indexing?**

> O(n × k) where n = number of files, k = average filename length (for `strings.ToLower` + map insertion). Map insertion is O(1) amortized. `filepath.Walk` is O(n) filesystem calls.

**Q21: What's the space complexity?**

> O(n × (k + p)) where k = filename length, p = path length. For 10,000 files with avg 20-byte names and 80-byte paths, that's ~1MB. Plus map overhead (~100 bytes per entry) → ~2MB total.

**Q22: How would you handle 1 million files?**

> 1. Current approach: ~200MB memory, ~5-10s indexing. JSON serialization would be ~100MB. Search still O(1).
> 2. Improvement: Use a database (BoltDB, SQLite) for persistence. Stream results instead of loading all into memory.
> 3. Use `filepath.WalkDir` (avoids `Lstat` — 30-50% faster).
> 4. Parallelize indexing with goroutines + channel (fan-out pattern).

**Q23: How would you add content search?**

> During `IndexFiles`, open each file, read line by line, tokenize into words, and store `word → []FileInfo` in an inverted index. Use `bufio.Scanner` for memory-efficient line reading. Consider mmap for large files.

**Q24: What's an inverted index?**

> A data structure that maps tokens (words) to the documents (files) containing them. Like a book index: "chapter 3 → pages 45, 67." In this project, the current `map[string]string` is a simplified version — mapping filenames to paths. A true inverted index would map content tokens to file lists.

**Q25: Why not use a database like SQLite?**

> For a CLI tool, a JSON file is simpler, has no dependencies, and is human-readable. SQLite would add a dependency but provide better performance for large datasets, concurrent access, and partial queries. For production use, SQLite is better.

**Q26: How do you handle concurrent access to `fileIndex`?**

> Currently: I don't. The map is only written during indexing (single-threaded) and read during search. If I parallelized indexing, I'd need `sync.Mutex` or `sync.Map`. Go maps are not safe for concurrent writes.

**Q27: What happens if two instances run simultaneously?**

> Race condition on both the in-memory map and the `index.json` file. One instance could overwrite the other's index. Fix: file locking (`syscall.Flock`) or atomic writes (write to temp file, then `os.Rename`).

**Q28: Explain your error handling strategy.**

> Print and continue. All errors are logged to stdout with `fmt.Println`. No errors are propagated to the caller. This is acceptable for a CLI tool but not for a library. In a library, I'd return `error` values.

**Q29: How would you add logging?**

> Replace `fmt.Println` with `log` package or `slog` (Go 1.21+). Add log levels (INFO, WARN, ERROR). Optionally write to a file. For a CLI tool, `log.SetFlags(log.Ldate | log.Ltime)` is sufficient.

**Q30: How would you add rate limiting?**

> Not applicable for a local CLI tool. If this were a web service, I'd use `golang.org/x/time/rate` (token bucket) or a middleware like `httputil/throttler`.

---

## SECTION 4 — AI/LLM Questions

*Not directly applicable — your project doesn't use AI. But interviewers may ask:*

**Q31: How would you add AI-powered semantic search to SwiftSearch?**

> 1. During indexing, compute embeddings for file names and content snippets using OpenAI's `text-embedding-3-small`.
> 2. Store embeddings in a vector database (ChromaDB, Pinecone, or pgvector).
> 3. At query time, compute the query embedding, then do cosine similarity search against stored embeddings.
> 4. Return top-k results ranked by similarity score.

**Q32: What would be the cost of embedding 10,000 files?**

> At ~$0.02/1M tokens (text-embedding-3-small), assuming 500 tokens per file: 10,000 × 500 = 5M tokens → ~$0.10. Very cheap.

**Q33: How would you handle the context window limitation?**

> Chunk files into 512-token segments with 50-token overlap. Store each chunk as a separate embedding. At query time, retrieve top chunks and reassemble context.

---

## SECTION 5 — Docker Questions

**Q34: How would you Dockerize this?**

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN go build -o swiftsearch .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/swiftsearch .
ENTRYPOINT ["./swiftsearch"]
```

**Q35: Why multi-stage builds?**

> The build stage includes the Go compiler (~800MB). The runtime stage is just the binary (~5MB) + Alpine (~5MB). Multi-stage reduces the final image from ~800MB to ~10MB.

**Q36: Why Alpine?**

> Smallest Linux distro (~5MB). No bloat. `alpine:3.19` is the current stable. Alternative: `scratch` (empty image, ~0MB, but no shell or utilities).

**Q37: How would you use Docker Compose?**

> Not really needed for a CLI tool. But if you wanted to run it as a service:
```yaml
services:
  swiftsearch:
    build: .
    volumes:
      - /host/data:/data
    command: ["-index", "-dir", "/data"]
```

**Q38: How would you deploy this in production?**

> 1. Build the binary with `CGO_ENABLED=0 GOOS=linux go build -o swiftsearch`
> 2. Copy to server via SCP or store in S3
> 3. Or: Push Docker image to ECR/GCR, run on ECS/Kubernetes
> 4. For a CLI tool, distributing the binary directly is simplest

---

## SECTION 6 — Go-Specific Questions

**Q39: Why Go and not Python/Node.js/Rust?**

> Go provides: compiled binary (no runtime dependency), fast execution, built-in concurrency (goroutines/channels), excellent standard library (flag, filepath, encoding/json), cross-compilation. Python would be 10-100x slower for large directories. Rust would be faster but more complex to write. Node.js has weaker filesystem support.

**Q40: Explain goroutines and channels.**

> Goroutines are lightweight threads managed by Go's runtime (~2KB initial stack, grows as needed). Thousands can run concurrently. Channels are typed pipes for communication between goroutines. `ch <- val` sends, `val := <-ch` receives. Unbuffered channels block until both sender and receiver are ready.

**Q41: What's the difference between `make(chan string)` and `make(chan string, 1)`?**

> Unbuffered (`make(chan string)`) blocks until both sides are ready. Buffered (`make(chan string, 1)`) allows one send without a receiver. In this project, the unbuffered channel means the goroutine blocks until the main goroutine reads.

**Q42: What is `sync.Map` and when would you use it?**

> `sync.Map` is a concurrency-safe map optimized for two patterns: 1) keys written once but read many times, 2) disjoint sets of keys. It uses a read-only map + dirty map internally. For this project, `sync.Map` would be appropriate if indexing were parallelized.

**Q43: What's `filepath.Walk` vs `filepath.WalkDir`?**

> `Walk` calls `os.Lstat` for every file entry (extra syscall). `WalkDir` uses `DirEntry` which can provide the type from the directory listing without an extra stat. `WalkDir` is 30-50% faster.

**Q44: How does Go handle memory management?**

> Garbage collector (GC). Go uses a tri-color mark-and-sweep concurrent GC. Pauses are typically <1ms. For this project, the map is the primary allocation. The GC handles cleanup when the program exits.

**Q45: What's the difference between `var x = make(map[string]string)` and `var x map[string]string`?**

> `make` initializes the map (allocates internal hash table). The second declaration is `nil` — reading from it returns zero value, but writing causes a panic.

---

## SECTION 7 — System Design

**Q46: How would this work with 100 users?**

> As a CLI tool, it's per-user. 100 users = 100 instances. No shared state. Each user has their own `index.json`. Works fine.

**Q47: How about 10,000 concurrent users?**

> If converted to a web service:
> - Replace global state with per-request state or a shared concurrent-safe store
> - Add an HTTP API: `GET /search?q=...`
> - Use a worker pool for indexing (N goroutines processing file chunks)
> - Cache index in Redis for fast access
> - Use a database (PostgreSQL) for persistent storage

**Q48: How about 1 million users?**

> Full distributed system:
> - Load balancer (nginx/ALB) → multiple Go API servers
> - Shared index store (Elasticsearch for full-text, Redis for hot data)
> - Message queue (Kafka) for async indexing jobs
> - CDN for static assets
> - Database sharding by directory/project
> - Horizontal scaling: add more API servers as load increases

**Q49: How would you add caching?**

> In-memory LRU cache for recent queries. In Go: `container/list` + map (or `github.com/hashicorp/golang-lru`). Cache key: query string, value: result paths. TTL: re-index invalidates cache.

**Q50: How would you add a REST API?**

> Use `net/http` or `gin`:
```go
http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    results := SearchFromIndex(query)
    json.NewEncoder(w).Encode(results)
})
```

**Q51: How would you handle authentication?**

> JWT tokens in `Authorization` header. Middleware validates token, extracts user ID, checks permissions. For a local CLI tool, OS-level file permissions are sufficient.

---

## SECTION 8 — Production Readiness

**Q52: How do you monitor this?**

> For CLI: exit codes (0 = success, 1 = error). For web service: Prometheus metrics (search latency, index size, error rate), health check endpoint, structured logging.

**Q53: How do you handle failures?**

> Graceful degradation: if index is corrupted, re-index. If file is unreadable, skip it. Always print actionable error messages. For web service: circuit breakers, retries with exponential backoff.

**Q54: Where do you store secrets?**

> This project has no secrets. For a web service: environment variables, HashiCorp Vault, AWS Secrets Manager. Never commit to git.

**Q55: How would you set up CI/CD?**

> GitHub Actions:
1. `go vet ./...` — static analysis
2. `go test ./...` — unit tests
3. `golangci-lint run` — linting
4. `go build` — compilation check
5. `docker build` — image build
6. Push to registry on main branch

**Q56: How would you test this?**

```go
func TestIndexFiles(t *testing.T) {
    dir := t.TempDir()
    os.Create(filepath.Join(dir, "test.txt"))
    IndexFiles(dir)
    if _, ok := fileIndex["test.txt"]; !ok {
        t.Error("expected test.txt in index")
    }
}
```

**Q57: What security concerns exist?**

> 1. Index file is world-readable (0644) — could leak sensitive file paths
> 2. No input validation on directory path — could index `/etc/passwd`
> 3. No size limits — indexing `/` would consume all memory
> 4. Path traversal in search queries (not currently exploitable since it's exact match)

---

## SECTION 9 — Behavioral Questions

**Q58: What was the hardest bug?**

> The deadlock at commit `2d1044a`. The channel-based search was blocking because the goroutine was sending to a channel nobody was reading from. The fix was ensuring the channel receive happens immediately after spawning the goroutine.

**Q59: What did you learn?**

> That concurrency adds complexity. A simple map lookup became a goroutine + channel pattern, introducing deadlock risk. Now I ask: "Does this actually need concurrency?" before adding goroutines.

**Q60: Why this tech stack?**

> Go for performance and simplicity. Zero dependencies for portability. JSON for human readability. Standard library for everything — no vendor lock-in.

**Q61: What would you do differently?**

> Start with a simpler design (no channels), then add concurrency only if benchmarks show it's needed. Write tests from the start. Match README claims to actual implementation.

---

## SECTION 10 — Deep Dive ("Why?" Chain)

**Q62: Why a map?** → O(1) lookup.

**Q63: Why not a trie?** → No prefix search needed for exact match. Trie adds complexity.

**Q64: Why not a database?** → No dependencies, simpler deployment.

**Q65: Why JSON for persistence?** → Human-readable, stdlib support.

**Q66: Why not protobuf?** → Overkill for a CLI tool, not human-readable.

**Q67: Why `strings.ToLower` instead of a case-sensitive map?** → Users expect case-insensitive search.

**Q68: Why not `filepath.WalkDir`?** → The code was written before `WalkDir` was commonly known. It was added in Go 1.16.

**Q69: Why the channel?** → Initial design intent for streaming results. Currently unnecessary.

**Q70: Why not return an error?** → CLI tool prints errors inline. Returning errors would add complexity without benefit for this scope.

---

## SECTION 11 — Follow-up Questions

For every answer above, here are follow-ups an interviewer would ask:

- "You said O(1) — is that truly O(1) or O(k) where k is key length?" → O(k) for hash computation, but k is bounded by filename length, so practically O(1).
- "You said the channel is unnecessary — why did you keep it?" → It was part of the original design. Refactoring it out is on the roadmap.
- "You said errors are printed — what if stdout is piped to a file?" → Errors mix with output. Should use stderr (`fmt.Fprintln(os.Stderr, ...)`).
- "You said the index is saved — what if the disk is full?" → `os.WriteFile` returns an error, which is printed. The in-memory index is still valid for the current session.

---

## SECTION 12 — Mock Interview (60 minutes)

**Minute 0-5: Introduction**
1. Tell me about yourself and this project.

**Minute 5-15: Architecture**
2. Draw the architecture on a whiteboard (describe it).
3. Walk me through what happens when I run `./swiftsearch -index -dir ./src`.
4. Walk me through what happens when I run `./swiftsearch -search main.go`.

**Minute 15-25: Code Deep Dive**
5. What's the time and space complexity of indexing?
6. What happens if two files have the same name?
7. Why did you use a goroutine for search?
8. What's the deadlock bug you fixed?
9. How would you add content search?

**Minute 25-35: Systems Design**
10. How would you convert this to a web service?
11. How would you handle 10,000 concurrent users?
12. How would you scale to 1 million files?

**Minute 35-45: Code Review**
13. What would you change about this code?
14. What tests would you write?
15. How would you Dockerize this?

**Minute 45-55: Behavioral**
16. What was the hardest part?
17. What did you learn?
18. What would you do differently?

**Minute 55-60: Questions**
19. What questions do you have for me?

---

## SECTION 13 — Hidden / Tricky Questions

These are questions interviewers ask to verify you actually built the project:

**Q71: What's in your `index.json` right now? Can you show me the structure?**

> It's a flat JSON object: `{"filename": "full/path", ...}`. Keys are lowercased filenames, values are the relative paths. 46 entries including `.git` internals.

**Q72: Your index.json includes `.git` entries. Why?**

> Because `filepath.Walk` doesn't skip hidden directories by default. I didn't add filtering. The index includes `.git/hooks`, `.git/config`, etc. This is a known limitation.

**Q73: If I search for "hooks", what happens?**

> It would find `.git/hooks` because the key is `"hooks"` and the path is `.git/hooks`. But searching for `".git"` would also return `.git` as a directory.

**Q74: What's the file permission on `index.json`?**

> `0644` — owner read/write, group and others read-only. Set by `os.WriteFile`.

**Q75: Your go.mod says `go 1.25.1`. Isn't that unreleased?**

> Yes, Go 1.25 is a future version. The go.mod was likely set during development with a pre-release toolchain. In an interview, acknowledge this — "I was using the latest available toolchain during development."

**Q76: What happens if I run `-search` before `-index`?**

> `LoadIndex` is called first. If `index.json` doesn't exist, it prints "No index found" and returns. `fileIndex` remains empty. The search runs but finds nothing. The user gets "Not found in index." This is a poor UX — it should exit with an error.

**Q77: Your channel is global. What if someone calls `SearchFromIndex` twice?**

> First call sends to the channel, second call also sends. But only one receive happens in `cli.go`. The second send blocks forever → deadlock. This is a real bug if the function is ever called twice.

**Q78: Why `fmt.Println("🔹", result)` with an emoji?**

> It's a visual marker in the terminal. Stylistic choice. In production, you'd use a proper output formatter.

**Q79: What's the difference between your project and `find . -name "main.go"`?**

> `find` traverses the filesystem every time. SwiftSearch builds an index once and reuses it. For repeated searches on the same directory, SwiftSearch is faster after the first index. For one-off searches, `find` is faster because there's no indexing overhead.

**Q80: Could this project cause a denial of service?**

> If run on `/` (root directory), it would try to index every file on the system, potentially consuming all memory and disk I/O. There's no depth limit, file count limit, or size limit.

---

## SECTION 14 — Resume Cross-check

If your resume says things like:
- "Built a concurrent file search engine in Go" — you can defend the goroutine+channel usage, but be honest that the concurrency is minimal
- "Implemented an inverted index" — it's a flat map, not a traditional inverted index (which maps tokens to documents). An inverted index would be needed for content search
- "Fan-in/fan-out pattern" — the current code doesn't implement this. You'd need parallel `filepath.Walk` goroutines feeding results into a single channel
- "Real-time streaming results" — only one result is returned, not streamed
- "Sub-second search on large directories" — true for the map lookup, but the README claims features (content search, dual-mode matching) that don't exist

**Be honest in interviews:** "The README is aspirational — it describes the intended final product. The current implementation does X." Interviewers respect honesty over bluffing.

---

## SECTION 15 — FINAL REVISION SHEET

### 100 Most Likely Interview Questions

#### Project Understanding (1-10)
1. What does SwiftSearch do? — CLI tool that indexes filenames into a map for O(1) exact-match lookup
2. Explain the architecture — 3 files: entry point, CLI parsing, indexing/search
3. Walk through the request flow — flags → parse → load index → index or search
4. Why did you build this? — Demonstrate Go concurrency, CLI design, filesystem traversal
5. What's the hardest part? — Deadlock bug with channel-based search
6. What would you redesign? — Remove channel, fix duplicate filenames, add content search
7. What's the biggest bug? — Duplicate filenames silently overwrite in the map
8. What would you improve? — Partial matching, content indexing, error propagation
9. Why Go? — Compiled binary, stdlib, concurrency, cross-compilation
10. What's the project's biggest limitation? — Exact-match only, filename only, single file per name

#### Data Structures (11-20)
11. Why a map? — O(1) average lookup, simple, stdlib
12. Time complexity of indexing? — O(n × k) where k = filename length
13. Space complexity? — O(n × (filename + path length))
14. Why not a trie? — No prefix search needed, adds complexity
15. Why not a database? — Zero dependencies, simpler deployment for CLI
16. What's an inverted index? — Maps tokens to documents. Current map is simplified version
17. How to handle duplicate filenames? — Change to `map[string][]string`
18. Map vs sync.Map? — sync.Map for concurrent writes, regular map for single-threaded
19. What if index has 1M entries? — ~200MB memory, still O(1) lookup
20. JSON vs protobuf? — JSON for human readability, protobuf for speed/size

#### Go Concurrency (21-30)
21. What are goroutines? — Lightweight threads (~2KB stack), managed by Go runtime
22. What are channels? — Typed pipes for goroutine communication
23. Buffered vs unbuffered channels? — Unbuffered blocks until both sides ready, buffered allows N sends without receiver
24. Why did you use a goroutine? — Original design for streaming results. Currently unnecessary
25. What's a deadlock? — Goroutine waiting on channel that will never receive/send
26. How to prevent deadlocks? — Ensure channel operations are paired, use timeouts with `select`
27. What's `select`? — Multiplexes channel operations, like switch for channels
28. How would you parallelize indexing? — Fan-out: N goroutines each walk a subdirectory, fan-in: single goroutine merges results
29. What's a mutex? — `sync.Mutex` for protecting shared state in concurrent code
30. What's a WaitGroup? — `sync.WaitGroup` for waiting on multiple goroutines to finish

#### Go Language (31-40)
31. Why `flag` package? — Stdlib, zero deps, sufficient for simple CLIs
32. What's `defer`? — Deferred function call, runs when surrounding function returns
33. What's `init()`? — Package initialization function, runs before `main`
34. Error handling pattern? — `if err != nil { return err }` — Go's explicit error handling
35. What's `interface{}`? — Empty interface, any type satisfies it. Now written as `any`
36. What's type assertion? — `x.(Type)` to extract concrete type from interface
37. What's `go mod`? — Go module management. Defines module path and dependencies
38. What's CGO? — Go's C interop. Disabled by default for cross-compilation
39. What's `go vet`? — Static analysis tool for suspicious constructs
40. What's `golangci-lint`? — Meta-linter combining multiple Go linters

#### CLI Design (41-48)
41. Why flags instead of positional args? — Explicit, self-documenting, supports defaults
42. How to support subcommands? — `flag.NewFlagSet` or use `cobra` library
43. How to add `--help`? — `flag.Usage` or custom help function
44. How to handle invalid flags? — `flag.Parse()` returns error, or check `flag.Visit`
45. Why return from each branch? — Prevent fall-through in if-chain
46. How to add colored output? — ANSI escape codes or `fatih/color` library
47. How to make it pipe-friendly? — Output to stdout, errors to stderr, no emojis
48. How to add shell completion? — Generate bash/zsh/fish completion scripts

#### Filesystem (49-56)
49. `filepath.Walk` vs `os.ReadDir`? — Walk is recursive, ReadDir is one level
50. `filepath.Walk` vs `filepath.WalkDir`? — WalkDir avoids extra Lstat syscall, 30-50% faster
51. How to skip hidden files? — Check `strings.HasPrefix(name, ".")` in Walk callback
52. How to skip binary files? — Check file extension or read first 512 bytes for null bytes
53. How to handle symlinks? — `filepath.Walk` follows symlinks by default (can cause loops)
54. How to handle large files? — Stream with `bufio.Scanner`, don't load entire file
55. How to watch for changes? — `fsnotify` library for inotify/kqueue events
56. File permissions? — `0644` = owner rw, group r, others r. `0755` for executables

#### Testing (57-64)
57. How to test `IndexFiles`? — Create temp dir, add files, call IndexFiles, assert map contents
58. How to test `SearchFromIndex`? — Populate map, call function, verify channel receives correct value
59. How to test CLI? — Use `os.Args` manipulation + capture stdout/stderr
60. Table-driven tests? — `[]struct{ input, expected }` loop, idiomatic Go testing
61. How to test errors? — Mock filesystem with `os.MkdirTemp` + `os.Create`
62. Benchmark tests? — `func BenchmarkXxx(b *testing.B)` with `b.N` iterations
63. Test coverage? — `go test -cover ./...` shows percentage
64. Integration tests? — Test full flow: index → save → load → search

#### Error Handling (65-72)
65. Why print errors instead of returning? — CLI tool, user-facing, simple scope
66. What's error wrapping? — `fmt.Errorf("context: %w", err)` preserves original error
67. What's `errors.Is` and `errors.As`? — Check error chain for specific error types
68. How to handle `filepath.Walk` errors? — Return nil to continue, return error to abort
69. What if `json.Unmarshal` fails? — Corrupted index, user should re-index
70. How to make errors actionable? — "Index not found. Run with -index first."
71. Should errors go to stdout or stderr? — stderr, so they don't mix with search results
72. What's the `panic` vs `error` tradeoff? — panic for unrecoverable bugs, error for expected failures

#### Performance (73-80)
73. Where are the bottlenecks? — Filesystem I/O during indexing, JSON serialization
74. How to optimize indexing? — Parallel walks, WalkDir, skip hidden/binary files
75. How to optimize search? — Already O(1), can't improve
76. How to optimize JSON serialization? — Use `json.Marshal` (no indent) or protobuf
77. Memory profiling? — `runtime/pprof` or `net/http/pprof`
78. CPU profiling? — `go test -cpuprofile cpu.out`
79. Benchmark comparison? — `go test -bench=. -benchmem`
80. How to reduce memory? — Stream results, use `[]byte` instead of `string` where possible

#### Security (81-88)
81. Path traversal? — Not exploitable (exact match only), but would be with content search
82. Symlink attacks? — `filepath.Walk` follows symlinks, could cause infinite loops
83. Denial of service? — No limits on directory size, could exhaust memory
84. Information leakage? — Index file is world-readable, contains all file paths
85. Input validation? — None currently. Should validate directory exists and is accessible
86. Race conditions? — No file locking on `index.json`
87. Secrets in code? — None. No API keys, no credentials
88. Supply chain? — Zero external dependencies. No supply chain risk

#### Docker & Deployment (89-96)
89. Multi-stage build? — Build stage has Go compiler (~800MB), runtime has only binary (~5MB)
90. Why Alpine? — Smallest Linux distro, ~5MB
91. Cross-compilation? — `GOOS=linux GOARCH=amd64 go build` for any target
92. Static binary? — `CGO_ENABLED=0` disables CGO, produces fully static binary
93. Docker Compose? — Not needed for CLI, but useful for web service version
94. Kubernetes deployment? — Deployment + Service + ConfigMap
95. CI/CD pipeline? — lint → test → build → push image → deploy
96. Blue-green deployment? — Two identical environments, switch traffic atomically

#### Behavioral (97-100)
97. Biggest challenge? — Deadlock bug, learned when concurrency is actually needed
98. Biggest mistake? — README claims features that aren't implemented yet
99. What did you learn? — Simplicity over cleverness. Map lookup doesn't need goroutines
100. What would you tell a junior dev? — "Don't add concurrency unless benchmarks prove you need it"

---

## CHEAT SHEET — Copy This Tonight

```
SwiftSearch:
- 3 files, 119 lines, 0 dependencies
- main.go → HandleCLI()
- cli.go → flag parsing, branch to index/search
- indexer.go → map[string]string, Walk, JSON persistence

Data flow:
  CLI flags → Parse → LoadIndex() → IndexFiles() or SearchFromIndex()

Key data structures:
  fileIndex: map[string]string  (lowercase_name → path)
  IndexResultChannel: chan string (unbuffered)

Complexities:
  Indexing: O(n × k) time, O(n × (k+p)) space
  Search: O(1) average, O(n) worst case (hash collision)

Bugs/Limitations:
  1. Duplicate filenames → last one wins (silent data loss)
  2. Exact match only → no partial/fuzzy
  3. Filename only → no content search
  4. Channel is global/non-resettable → deadlock on second use
  5. Goroutine for search is unnecessary overhead
  6. No stderr separation
  7. No file type filtering
  8. Indexes .git internals

README vs Code gaps:
  - "Parallel goroutines" → code is single-threaded
  - "Content search" → not implemented
  - "Fan-in/fan-out" → doesn't exist
  - "Dual-mode matching" → filename only
  - CLI flags (--name-only, --case-sensitive) → not implemented
```

---

## Things NOT to Say

1. **"It's a simple project"** — Say: "It's a focused project that demonstrates X, Y, Z"
2. **"The README describes the final product"** — Say: "The current implementation does X. The README outlines the roadmap"
3. **"I used a goroutine for performance"** — Say: "I used a goroutine for the channel-based result pattern, though I recognize it's unnecessary for a map lookup"
4. **"It has no bugs"** — Say: "I'm aware of the duplicate filename limitation and the channel reset issue"
5. **"I built everything from scratch"** — Say: "I used Go's standard library, which was sufficient for all functionality"
6. **"The concurrency makes it fast"** — Say: "The O(1) map lookup makes search fast. The concurrency is for the channel-based result delivery pattern"
7. **"I didn't think about security"** — Say: "I considered file permissions and error handling. For a local CLI tool, the attack surface is minimal"

---

## Common Mistakes to Avoid

1. Claiming the project does things the README describes but code doesn't implement
2. Saying "goroutines make it faster" when the goroutine adds overhead for a map lookup
3. Not knowing the deadlock bug and how you fixed it
4. Can't explain why duplicate filenames are a problem
5. Not knowing the time/space complexity
6. Saying "it's just a map" without explaining the design decisions
7. Not having a testing strategy
8. Not knowing what you'd do differently

---

## Red Flags Interviewers Look For

1. **Can't explain every line** — means you didn't write it
2. **Parrots README instead of code** — means you don't understand the gap
3. **Can't discuss alternatives** — means you chose this approach randomly
4. **No awareness of limitations** — means you haven't tested edge cases
5. **Claims "no bugs"** — means you haven't looked hard enough
6. **Can't discuss scaling** — means you only thought about the happy path
7. **No testing discussion** — means you didn't write tests
8. **Can't explain the deadlock** — means the "fixed a bug" commit isn't yours

---

## Architecture Summary (One-liner)

**SwiftSearch is a Go CLI that builds a `filename→path` hash map via `filepath.Walk`, persists it as JSON, and provides O(1) exact-match filename lookup using a (currently unnecessary) goroutine+channel pattern.**

---

## 30-Second Revision Summary

- **What:** Go CLI for instant filename search
- **How:** Hash map + JSON persistence
- **Key insight:** O(1) lookup after O(n) indexing
- **Key bug:** Duplicate filenames, channel deadlock
- **Key lesson:** Don't add concurrency unless needed
- **Key gap:** README promises more than code delivers
- **Key strength:** Zero dependencies, clean separation, idiomatic Go
