package main

import (
	"bufio"
	"fmt"
	std "github.com/balzaczyy/golucene/analysis/standard"
	_ "github.com/balzaczyy/golucene/core/codec/lucene49"
	docu "github.com/balzaczyy/golucene/core/document"
	"github.com/balzaczyy/golucene/core/index"
	"github.com/balzaczyy/golucene/core/search"
	"github.com/balzaczyy/golucene/core/store"
	"github.com/balzaczyy/golucene/core/util"
	"log"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run indexFile.go hamlet.go")
		return
	}

	start := time.Now()
	nRead := 0
	defer func() {
		log.Printf("Processed %v lines in %v.", nRead, time.Now().Sub(start))
	}()

	util.SetDefaultInfoStream(util.NewPrintStreamInfoStream(os.Stdout))
	index.DefaultSimilarity = func() index.Similarity {
		return search.NewDefaultSimilarity()
	}

	directory, err := store.OpenFSDirectory("app/index")
	if err != nil {
		log.Panicf("Failed to open directory: %v", err)
	}
	defer directory.Close()

	analyzer := std.NewStandardAnalyzer(util.VERSION_49)
	conf := index.NewIndexWriterConfig(util.VERSION_49, analyzer)
	writer, err := index.NewIndexWriter(directory, conf)
	if err != nil {
		log.Panicf("Failed to create writer: %v", err)
	}
	defer writer.Close() // ensure index is written

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Panicf("Failed to open file '%v': %v", os.Args[1], err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}
		nRead++

		d := docu.NewDocument()
		d.Add(docu.NewTextFieldFromString("text", text, docu.STORE_YES))
		if err = writer.AddDocument(d.Fields()); err != nil {
			log.Panicf("Failed to process line '%v': %v", text, err)
		}
	}
}
