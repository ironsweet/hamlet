package main

import (
	"encoding/json"
	"fmt"
	"github.com/balzaczyy/firebase"
	"github.com/balzaczyy/go-logging"
	std "github.com/balzaczyy/golucene/analysis/standard"
	_ "github.com/balzaczyy/golucene/core/codec/lucene410"
	"github.com/balzaczyy/golucene/core/index"
	"github.com/balzaczyy/golucene/core/search"
	"github.com/balzaczyy/golucene/core/store"
	"github.com/balzaczyy/golucene/core/util"
	qp "github.com/balzaczyy/golucene/queryparser/classic"
	"net/http"
	"os"
	"path"
	"strings"
)

var SILENT = map[string]string{
	"print": "silent",
}

type FirebaseLogger struct {
	queue  chan []byte
	closer chan chan error
}

func newLogger(root string) *FirebaseLogger {
	api := new(firebase.Client)
	api.Init(root, "", nil)
	queue := make(chan []byte, 1000)
	closer := make(chan chan error)
	go func() {
		isRunning := true
		for isRunning {
			select {
			case reply := <-closer:
				isRunning = false
				reply <- nil
			case b := <-queue:
				_, err := api.Push(string(b), SILENT)
				if err != nil {
					fmt.Println(err)
					fmt.Println(string(b))
				}
			}
		}
	}()
	return &FirebaseLogger{
		queue:  queue,
		closer: closer,
	}
}

func (logger *FirebaseLogger) Write(b []byte) (int, error) {
	logger.queue <- b
	return len(b), nil
}

func (logger *FirebaseLogger) Close() error {
	ok := make(chan error)
	logger.closer <- ok
	return <-ok
}

var log = logging.MustGetLogger("hamlet")

func main() {
	// setup logger
	logger := newLogger(
		"https://shining-inferno-3740.firebaseio.com/log/hamlet",
	)
	defer logger.Close()

	logging.SetBackend(logging.NewBackendFormatter(
		logging.NewLogBackend(logger, "", 0),
		logging.MustStringFormatter(
			"%{color}%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}",
		),
	))

	// setup index and searcher
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

	// setup web service
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
