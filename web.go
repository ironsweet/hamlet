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
	"github.com/op/go-logging"
	"net/http"
	"os"
	"path"
	"strings"
)

var log = logging.MustGetLogger("hamlet")

func init() {
	format := logging.MustStringFormatter(
		"%{color}%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
	backend := logging.NewLogBackend(os.Stdout, "", 0)
	logging.SetBackend(logging.NewBackendFormatter(backend, format))
	log.Info("Logger is configured.")
}

func main() {

	util.SetDefaultInfoStream(util.NewPrintStreamInfoStream(os.Stdout))
	index.DefaultSimilarity = func() index.Similarity {
		return search.NewDefaultSimilarity()
	}
	directory, err := store.OpenFSDirectory("index")
	if err != nil {
		log.Critical("Failed to open directory: %v", err)
		return
	}
	defer directory.Close()
	reader, err := index.OpenDirectoryReader(directory)
	if err != nil {
		log.Critical("Failed to open writer: %v", err)
		return
	}
	defer reader.Close()
	analyzer := std.NewStandardAnalyzer()
	qParser := qp.NewQueryParser(util.VERSION_49, "text", analyzer)
	ss := search.NewIndexSearcher(reader)

	port := os.Getenv("VCAP_APP_PORT")
	if port == "" {
		port = "8081"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Path
		if path.Ext(file) == "" {
			file = path.Join(file, "index.html")
		}
		if strings.HasPrefix(file, "/") {
			file = file[1:]
		}
		log.Debug("Serving %v", file)
		http.ServeFile(w, r, file)
	})
	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		queryStr := r.URL.Query().Get("q")
		log.Debug("Query: %v", queryStr)
		query, err := qParser.Parse(queryStr)
		if err != nil {
			log.Error("Parse failed: %v", err)
			w.WriteHeader(500)
			return
		}

		hits, err := ss.SearchTop(query, 100)
		if err != nil {
			log.Error("Search failed: %v", err)
			w.WriteHeader(500)
			return
		}

		var lines []string
		for _, hit := range hits.ScoreDocs {
			doc, err := reader.Document(hit.Doc)
			if err != nil {
				log.Warning("Fetch doc %v failed: %v", hit.Doc, err)
				continue
			}
			lines = append(lines, doc.Get("text"))
		}

		data, err := json.Marshal(lines)
		if err != nil {
			log.Error("IO error: %v", err)
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(200)
		w.Write(data)
	})
	log.Critical("%v", http.ListenAndServe(":"+port, nil))
}
