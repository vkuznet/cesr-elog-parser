package main

import (
	"flag"
	"log"
	"runtime"
)

func main() {
	inputDir := flag.String("input", "./logs", "Directory containing elog files")
	outputDir := flag.String("output", "./parsed", "Directory for NDJSON output files")
	workers := flag.Int("workers", runtime.NumCPU(), "Number of concurrent workers")
	ext := flag.String("ext", ".log", "File extension to process (e.g. .log, .elog, .txt)")
	qdrant := flag.String("qdrant", "", "qdrant URL, e.g. http://localhost:6333")
	col := flag.String("collection", "", "qdrant collection")
	flag.Parse()
	process(*inputDir, *outputDir, *ext, *workers)
	if *qdrant != "" {
		err := inject2Qdrant(*qdrant, *col, *outputDir)
		if err != nil {
			log.Fatalf(err.Error())
		}
	}
}
