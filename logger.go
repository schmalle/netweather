package main

import (
	"log"
	"os"
)

var logger *log.Logger

func initLogger(logFilePath string) {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	logger = log.New(file, "netweather: ", log.LstdFlags)
}
