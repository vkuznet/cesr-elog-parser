package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	inputDir := flag.String("input", "./logs", "Directory containing elog files")
	outputDir := flag.String("output", "./parsed", "Directory for NDJSON output files")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of concurrent workers")
	ext := flag.String("ext", ".log", "File extension to process (e.g. .log, .elog, .txt)")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("cannot create output dir: %v", err)
	}

	files, err := collectFiles(*inputDir, *ext)
	if err != nil {
		log.Fatalf("cannot collect files: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no *%s files found in %s", *ext, *inputDir)
	}

	log.Printf("found %d files, starting %d workers", len(files), *workers)
	start := time.Now()

	jobs := make(chan string, *workers*2)
	var (
		wg      sync.WaitGroup
		success atomic.Int64
		failed  atomic.Int64
	)

	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				outPath := filepath.Join(*outputDir, baseName(path)+".ndjson")
				if err := processFile(path, outPath); err != nil {
					log.Printf("ERROR %s: %v", path, err)
					failed.Add(1)
				} else {
					success.Add(1)
				}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(start)
	log.Printf("done in %s — success=%d failed=%d",
		elapsed.Round(time.Millisecond), success.Load(), failed.Load())

	if failed.Load() > 0 {
		fmt.Fprintf(os.Stderr, "%d files failed to parse\n", failed.Load())
		os.Exit(1)
	}
}

func collectFiles(dir, ext string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ext {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// baseName strips directory and extension.
func baseName(path string) string {
	return filepath.Base(path[:len(path)-len(filepath.Ext(path))])
}
