# ⚡ SwiftSearch

![Go](https://img.shields.io/badge/Go-Concurrent-blue?logo=go)

SwiftSearch is a blazing-fast **Command Line Search Tool** built with Go.  
It recursively scans directories, builds an **inverted index**, and instantly finds files matching your query — both in **file names and file contents**.

Perfect for developers, sysadmins, and anyone tired of slow searches! ⚙️🔥

---

##  Features

-  **Instant Search:** Sub-second lookup via in-memory inverted index
-  **Dual-Mode Matching:** Filename + text content search
-  **Smart Indexing:** Optimized recursive scanning for large directories
-  **Lightweight CLI:** No external dependencies — single binary deployment
-  **Cross-Platform:** Works on **Windows**, **macOS**, and **Linux**

---

## ⚙️ Concurrency Design

SwiftSearch is built with **Go’s powerful concurrency model**:

1) Multiple directory walkers run **in parallel** using Goroutines  
2) As soon as a match is found, it's pushed to the user through a **Result Channel**  
3) No need to wait for the full scan — results appear **streamingly in real-time**

## 🛠️ Installation

SwiftSearch requires **Go 1.19+**.

This architecture drastically reduces search latency — especially in **large & deep directory trees**.

---

## 🧬 How It Works

1️) **Concurrent Directory Scan**  
Recursive + parallel traversal using Goroutines

2️) **Index Phase**  
Creates a fast lookup structure:  
`word/filename → list of file paths`

3️) **Channel-based Query Result Streaming**  
Matches are sent instantly without blocking

4️) **Smart Output**  
Collects results from channels & prints them as soon as found

> ⏱️ Initial indexing may take a few seconds for large projects —  
> but subsequent searches become **instant and scalable**!

---

##  Technologies / Concepts Used

| Concept | Purpose |
|--------|---------|
| **Goroutines** | Parallel file scanning |
| **Channels** | Async result passing |
| **Inverted Index** | Instant search lookups |
| **Filesystem Walkers** | Efficient directory traversal |
| **Content Tokenization** | Text search inside files |
| **Concurrency Safety** | Fast, lock-free in-memory structure |

---


### Quick Setup

```bash
# Clone the repository
git clone https://github.com/<your-username>/SwiftSearch.git

# Enter project directory
cd SwiftSearch

# Build the binary
go build -o swiftsearch
🏁 Optional: Add to PATH

bash
Copy code
# macOS / Linux
sudo mv swiftsearch /usr/local/bin/
 Pre-built binaries coming soon (Releases page)
```
##  Usage
Run from terminal with a directory + search query:

bash
Copy code
./swiftsearch <directory> "<query>"
Examples
Basic content search

bash
Copy code
./swiftsearch ~/Documents "error handling"
Filename only search

bash
Copy code
./swiftsearch . "config.yaml" --name-only
Case-insensitive (default)

bash
Copy code
./swiftsearch src "TODO"
🔧 CLI Flags
Flag	Description	Default
--name-only	Search only filenames (skip content)	
--case-sensitive	Enable case-sensitive search	
--max-results N	Limit number of results	10
--help	Show help commands	-

## Full help:

bash
Copy code
./swiftsearch --help
How It Works
SwiftSearch is designed for speed + simplicity:

1️) Scan Phase
Recursive directory walker → reads files line-by-line

2️) Index Phase
Creates inverted index →
word -> list of file paths

3️) Query Phase
Instant lookup, no rescanning

4️) Output Phase
Displays:
✅ Matching file paths
✅ Line numbers
✅ Snippet previews (WIP)

💡 Ideal for repeated searches inside the same directory
⏱️ Example: 10–30s initial indexing → instant future queries

## 🎯 Roadmap

 Core search engine

 Regex search support

 Ignore binary / large files

 Persistent local index cache

 Fuzzy matching (Levenshtein)

 TUI (Terminal UI) interface

 Multi-threaded file parsing

 Ranked search scoring

## 🤝 Contributing
PRs are welcome!
Focus areas include:

✅ Performance improvements
✅ File-type filtering
✅ CI/CD & testing

bash
Copy code
Fork → Improve → Pull Request ✅
📜 License
Licensed under the MIT License
See LICENSE file for more details.

⭐ If SwiftSearch saves you time — consider giving the repo a star!
Questions? Open an issue or reach out on X/Twitter: @yourhandle

Happy searching! 🕵️‍♂️⚡
