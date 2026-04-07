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
	qdrant := flag.String("qdrant-url", "", "qdrant URL, e.g. http://localhost:6333")
	col := flag.String("qdrant-col", "", "qdrant collection")
	dim := flag.Int("qdrant-dim", 384, "qdrant collection dimension")
	flag.Parse()
	process(*inputDir, *outputDir, *ext, *workers)
	if *qdrant != "" && *col != "" {
		injectRAGs(*outputDir, *qdrant, *col, *dim)
	}
}
