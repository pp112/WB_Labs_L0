package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	stan "github.com/nats-io/stan.go"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run publish.go <file_or_directory_path>")
	}

	path := os.Args[1]

	// Подключаемся к NATS Streaming
	sc, err := stan.Connect("test-cluster", "test-pub", stan.NatsURL("nats://nats-streaming:4222"))
	if err != nil {
		log.Fatalf("failed to connect to NATS Streaming: %v", err)
	}
	defer sc.Close()

	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("failed to stat path: %v", err)
	}

	if info.IsDir() {
		// Если папка, проходим по всем JSON файлам
		err := filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
				return nil
			}
			return publishFile(sc, filePath)
		})
		if err != nil {
			log.Fatalf("error walking directory: %v", err)
		}
	} else {
		// Один файл
		if !strings.HasSuffix(strings.ToLower(path), ".json") {
			log.Fatalf("file is not a JSON: %s", path)
		}
		err = publishFile(sc, path)
		if err != nil {
			log.Fatalf("error publishing file: %v", err)
		}
	}

	log.Println("all files published successfully")
}

func publishFile(sc stan.Conn, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	err = sc.Publish("orders", data)
	if err != nil {
		return fmt.Errorf("failed to publish file %s: %w", filePath, err)
	}

	log.Printf("published %s\n", filePath)
	return nil
}
