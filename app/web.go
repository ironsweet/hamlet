package main

import (
	"encoding/json"
	std "github.com/balzaczyy/golucene/analysis/standard"
	_ "github.com/balzaczyy/golucene/core/codec/lucene49"
	"github.com/balzaczyy/golucene/core/index"
	"github.com/balzaczyy/golucene/core/search"
	"github.com/balzaczyy/golucene/core/store"
	"github.com/balzaczyy/golucene/core/util"
	qp "github.com/balzaczyy/golucene/queryparser/classic"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

func main() {

	util.SetDefaultInfoStream(util.NewPrintStreamInfoStream(os.Stdout))
	index.DefaultSimilarity = func() index.Similarity {
		return search.NewDefaultSimilarity()
	}
	directory, err := store.OpenFSDirectory("index")
	if err != nil {
		log.Panicf("Failed to open directory: %v", err)
	}
	defer directory.Close()
	reader, err := index.OpenDirectoryReader(directory)
	if err != nil {
		log.Panicf("Failed to open writer: %v", err)
	}
	defer reader.Close()
	analyzer := std.NewStandardAnalyzer(util.VERSION_49)
	qParser := qp.NewQueryParser(util.VERSION_49, "text", analyzer)
	ss := search.NewIndexSearcher(reader)

	port := os.Getenv("VCAP_APP_PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Path
		if path.Ext(file) == "" {
			file = path.Join(file, "index.html")
		}
		if strings.HasPrefix(file, "/") {
			file = file[1:]
		}
		log.Printf("Serving %v", file)
		http.ServeFile(w, r, file)
	})
	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		queryStr := r.URL.Query().Get("q")
		log.Printf("Query: %v", queryStr)
		query, err := qParser.Parse(queryStr)
		if err != nil {
			log.Printf("Parse failed: %v", err)
			w.WriteHeader(500)
			return
		}

		hits, err := ss.SearchTop(query, 100)
		if err != nil {
			log.Printf("Search failed: %v", err)
			w.WriteHeader(500)
			return
		}

		var lines []string
		for _, hit := range hits.ScoreDocs {
			doc, err := reader.Document(hit.Doc)
			if err != nil {
				log.Printf("Fetch doc %v failed: %v", hit.Doc, err)
				continue
			}
			lines = append(lines, doc.Get("text"))
		}

		data, err := json.Marshal(lines)
		if err != nil {
			log.Printf("IO error: %v", err)
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(200)
		w.Write(data)
	})
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
