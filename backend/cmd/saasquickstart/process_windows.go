//go:build windows

package main

import (
	"fmt"
	"os"
)

func cmdStart() {
	fmt.Println("The 'start' command is not natively supported on Windows.")
	fmt.Println("Please run the backend and frontend separately using 'go run ./cmd/server' and 'npm run dev'.")
	os.Exit(1)
}

func cmdStop() {
	fmt.Println("The 'stop' command is not natively supported on Windows. Please close terminal tabs manually.")
	os.Exit(1)
}

func cmdRestart() {
	fmt.Println("The 'restart' command is not natively supported on Windows. Please close and reopen terminal tabs manually.")
	os.Exit(1)
}
