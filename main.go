package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"os"
	//"errors"
)
// 1: first we will make a function to search files

func searchFiles(address string, toFind string) bool {
	found := false
	err := filepath.Walk(address, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := strings.ToLower(info.Name())
		query := strings.ToLower(toFind)
		if strings.Contains(name, query) {
			fmt.Println("Found:", path)
			found = true
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}
	return found
}


func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <directory> <query>")
		return
	}

	dir := os.Args[1]
	query := os.Args[2]

	if searchFiles(dir, query) {
		fmt.Println("File found!")
	} else {
		fmt.Println("File not found.")
	}
}
