package main

import (
	"flag"
	"runtime"
)

func main() {
	inputDir := flag.String("input", "./logs", "Directory containing elog files")
	outputDir := flag.String("output", "./parsed", "Directory for NDJSON output files")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of concurrent workers")
	ext := flag.String("ext", ".log", "File extension to process (e.g. .log, .elog, .txt)")
	flag.Parse()
	process(*inputDir, *outputDir, *ext, *workers)
}
