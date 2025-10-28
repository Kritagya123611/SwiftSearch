package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var fileIndex = make(map[string]string)

var IndexResultChannel = make(chan string)

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
