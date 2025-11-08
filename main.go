package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 1: first we will make a function to search files
// 2: initially i have made a function that takes address and the thing to search and then searches it recursively 
// 3: now i have to add concurrency in it to make it more faster
// 4: for concurrency we will use goroutines and channels
// 5: first my plan is to more than one goroutine to search in different subdirectories 
// 6: each goroutine will have its own channel to send the results(if found only)
// 7: last i plan to use for select loop to listen to all the channels and this conccurrency pattern automatically
//    checks which channel has the output and we can read from it Hence finding the file faster

// main global channel to collect any results from all goroutines
var ResultChannel = make(chan string)

func searchFiles(address string, toFind string) {
	entries, err := os.ReadDir(address)
	if err != nil {
		return
	}
	for _, entry := range entries {
		fullPath := filepath.Join(address, entry.Name())
		if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(toFind)) {
			fmt.Println("Found:", fullPath)
			ResultChannel <- fullPath
		}
		if entry.IsDir() {
			go searchFiles(fullPath, toFind)
		}
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <directory> <query>")
		return
	}

	dir := os.Args[1]
	query := os.Args[2]

	fmt.Println("Indexing files...")
	IndexFiles(dir)
	fmt.Println("Indexing complete!")
	fmt.Println("testing error")
	go SearchFromIndex(query)

	result := <-IndexResultChannel
	fmt.Println("Result:", result)

//for testing webhook for fixflow ai
//testing 12345
//testing webhook failure again
//testing webhook failure again 2
//testing webhook failure again 3
//testing webhook failure again 4
//testing webhook failure again 5 to check
errorpart
}