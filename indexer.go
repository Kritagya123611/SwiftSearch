package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"encoding/json"
)

var fileIndex = make(map[string]string)

var IndexResultChannel = make(chan string)

const indexFile = "index.json"

func SaveIndex() {
	data, err := json.MarshalIndent(fileIndex, "", "  ")
	if err != nil {
		fmt.Println("Error saving index:", err)
		return
	}

	err = os.WriteFile(indexFile, data, 0644)
	if err != nil {
		fmt.Println("Error writing index file:", err)
		return
	}

	fmt.Println("Index saved to", indexFile)
}

func LoadIndex() {
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		fmt.Println("No index found. Please run with -index first.")
		return
	}

	data, err := os.ReadFile(indexFile)
	if err != nil {
		fmt.Println("Error reading index file:", err)
		return
	}

	err = json.Unmarshal(data, &fileIndex)
	if err != nil {
		fmt.Println("Error parsing index:", err)
		return
	}

	fmt.Println("Index loaded — entries:", len(fileIndex))
}

func IndexFiles(root string) { 
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		fileIndex[strings.ToLower(info.Name())] = path
		return nil
	})
}

func SearchFromIndex(toFind string) {
	toFind = strings.ToLower(toFind)
	if path, exists := fileIndex[toFind]; exists {
		fmt.Println("Found in index:", path)
		IndexResultChannel <- path
	} else {
		fmt.Println("Not found in index.")
	}
}
