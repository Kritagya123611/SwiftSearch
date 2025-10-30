package main

import (
	"flag"
	"fmt"
)

func HandleCLI() {
	indexCmd := flag.Bool("index", false, "Index directory")
	searchCmd := flag.String("search", "", "Search query")
	dirCmd := flag.String("dir", ".", "Directory to index")

	flag.Parse()

	// Load existing index
	LoadIndex()

	if *indexCmd {
		fmt.Println("Indexing:", *dirCmd)
		IndexFiles(*dirCmd)
		SaveIndex()
		fmt.Println("✅ Index updated.")
		return
	}

	if *searchCmd != "" {
		fmt.Println("Searching:", *searchCmd)
		SearchFromIndex(*searchCmd)
		for result := range IndexResultChannel {
			fmt.Println("🔹", result)
		}
		return
	}

	fmt.Println("Use:")
	fmt.Println("  swiftsearch -index -dir <folder>")
	fmt.Println("  swiftsearch -search <query>")
}
