package main

import (
	"fmt"
	"os"
	"strings"
)

func PrintUpper(s string) {
	result := strings.ToUpper(s)
	fmt.Println(result)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
