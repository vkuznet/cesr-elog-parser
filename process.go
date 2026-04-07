package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func process(inputDir, outputDir, ext string, workers int) {

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("cannot create output dir: %v", err)
	}

	files, err := collectFiles(inputDir, ext)
	if err != nil {
		log.Fatalf("cannot collect files: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no *%s files found in %s", ext, inputDir)
	}

	log.Printf("found %d files, starting %d workers", len(files), workers)
	start := time.Now()

	jobs := make(chan string, workers*2)
	var (
		wg      sync.WaitGroup
		success atomic.Int64
		failed  atomic.Int64
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				outPath := filepath.Join(outputDir, baseName(path)+".ndjson")
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

// helper function to collect files which follow symlinks
func collectFiles(dir, ext string) ([]string, error) {
	var files []string

	visited := make(map[string]bool) // prevent infinite loops

	var walk func(string) error
	walk = func(current string) error {
		realPath, err := filepath.EvalSymlinks(current)
		if err != nil {
			return err
		}

		if visited[realPath] {
			return nil
		}
		visited[realPath] = true

		entries, err := os.ReadDir(current)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			path := filepath.Join(current, entry.Name())

			info, err := entry.Info()
			if err != nil {
				return err
			}

			mode := info.Mode()

			if mode&os.ModeSymlink != 0 {
				// Resolve symlink
				target, err := filepath.EvalSymlinks(path)
				if err != nil {
					continue
				}

				targetInfo, err := os.Stat(target)
				if err != nil {
					continue
				}

				if targetInfo.IsDir() {
					if err := walk(target); err != nil {
						return err
					}
				} else if filepath.Ext(target) == ext {
					files = append(files, target)
				}
				continue
			}

			if info.IsDir() {
				if err := walk(path); err != nil {
					return err
				}
			} else if filepath.Ext(path) == ext {
				files = append(files, path)
			}
		}
		return nil
	}

	err := walk(dir)
	return files, err
}

// baseName strips directory and extension.
func baseName(path string) string {
	return filepath.Base(path[:len(path)-len(filepath.Ext(path))])
}

func injectRAGs(logDir, endpoint, col string, dim int) {
	entries := GetLogEntries(logDir, "ndjson")
	var docs []RAGDoc
	for _, e := range entries {
		docs = append(docs, ToRAGDoc(e))
	}
	ctx := context.Background()
	qcfg := Config{Endpoint: endpoint, Collection: col, Dimension: dim}
	if strings.Contains(endpoint, "http") {
		qcfg.Protocol = ProtocolHTTP
	} else {
		qcfg.Protocol = ProtocolGRPC
	}
	cfg := InjectConfig{
		QdrantCfg: qcfg,
		Docs:      docs,
		BatchSize: 100,
	}

	ndocs, err := Inject(ctx, cfg)

	if err != nil {
		log.Fatalf(err.Error())
	}
	log.Printf("successfully injected: %d docs into Qdrant DB: %+v", ndocs, qcfg)
}
