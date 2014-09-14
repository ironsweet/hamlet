package main

import (
	"bufio"
	"fmt"
	ac "github.com/balzaczyy/golucene/analysis/core"
	std "github.com/balzaczyy/golucene/analysis/standard"
	_ "github.com/balzaczyy/golucene/core/codec/lucene49"
	docu "github.com/balzaczyy/golucene/core/document"
	"github.com/balzaczyy/golucene/core/index"
	"github.com/balzaczyy/golucene/core/search"
	"github.com/balzaczyy/golucene/core/store"
	"github.com/balzaczyy/golucene/core/util"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run indexFile.go hamlet.go")
		return
	}

	start := time.Now()
	nRead, nWords := 0, 0
	defer func() {
		log.Printf("Processed %v lines and %v words in %v.", nRead, nWords, time.Now().Sub(start))
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

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Panicf("Failed to open file '%v': %v", os.Args[1], err)
	}
	defer f.Close()

	var word string
	var wordInLine int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}
		nRead++
		for _, w := range strings.Fields(text) {
			if w = stripWord(w); len(w) == 0 {
				continue // no valid alphabet
			}
			if _, ok := ac.ENGLISH_STOP_WORDS_SET[w]; ok {
				continue // ignore stop word
			}
			if strings.Contains(w, ",") {
				panic(w)
			}
			nWords++
			if rand.Intn(nWords) == 0 {
				word, wordInLine = w, nRead-1
			}
		}

		d := docu.NewDocument()
		d.Add(docu.NewTextFieldFromString("text", text, docu.STORE_YES))
		if err = writer.AddDocument(d.Fields()); err != nil {
			log.Panicf("Failed to process line '%v': %v", text, err)
		}
	}

	if err = writer.Close(); err != nil {
		log.Panicf("Failed to flush index: %v", err)
	}

	log.Printf("Testing selected keyword: %v", word)
	reader, err := index.OpenDirectoryReader(directory)
	if err != nil {
		log.Panicf("Failed to open writer: %v", err)
	}
	defer reader.Close()

	ss := search.NewIndexSearcher(reader)
	searchStart := time.Now()
	hits, err := ss.SearchTop(search.NewTermQuery(index.NewTerm("text", word)), 1000)
	if err != nil {
		log.Panicf("Failed to search top hits: %v", err)
	}
	log.Printf("Found %v hits in %v.", hits.TotalHits, time.Now().Sub(searchStart))
	var found bool
	for _, hit := range hits.ScoreDocs {
		if hit.Doc == wordInLine {
			found = true
		}
	}
	if !found {
		log.Panicf("Failed to found expected hit: %v", wordInLine)
	}
	doc, err := reader.Document(wordInLine)
	if err != nil {
		log.Panicf("Failed to obtain document '%v': %v", wordInLine, err)
	}
	var matched bool
	for _, w := range strings.Fields(doc.Get("text")) {
		w = stripWord(w)
		if word == w {
			matched = true
		}
	}
	if !matched {
		log.Panicf("Word '%v' not found in text '%v'.", word, doc.Get("text"))
	}
	log.Println("Index done and verified.")
}

func stripWord(w string) string {
	w = strings.ToLower(w)
	start := -1
	for i, v := range w {
		isAlpha := (v >= 'a' && v <= 'z')
		if start >= 0 && !isAlpha {
			return w[start:i]
		}
		if start < 0 && isAlpha {
			start = i
		}
	}
	if start < 0 {
		return ""
	}
	return w[start:]
}
