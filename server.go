package main

import (
	"compress/gzip"
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
)

var (
	port        = flag.String("port", "8080", "port to run the server on")
	http_client = &http.Client{}
)

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func makeGzipHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			fn(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzr := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		fn(gzr, r)
	}
}

func getRequest(url string) (req *http.Request) {
	req, _ = http.NewRequest("GET", url, nil)
	req.Header.Add("X-Proxy-Bypass", "yes")
	return
}

func ChunkerServer(w http.ResponseWriter, req *http.Request) {
	var scheme string
	if req.TLS == nil {
		scheme = "http"
	} else {
		scheme = "https"
	}
	host := req.Host
	uri := req.RequestURI
	url := scheme + "://" + host + uri
	resp, _ := http_client.Do(getRequest(url + "&header=1"))
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("X-Accel-Buffering", "no")
	io.Copy(w, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	lastid := "0"
	for {
		resp, _ = http_client.Do(getRequest(url + "&lastid=" + lastid))
		if resp.StatusCode != 200 {
			return
		}
		n, _ := io.Copy(w, resp.Body)
		resp.Body.Close()
		if n == 0 {
			break
		}
		lastid = resp.Header.Get("LASTID")
		if lastid == "" {
			break
		}
	}
	resp, _ = http_client.Do(getRequest(url + "&footer=1"))
	if resp.StatusCode != 200 {
		return
	}
	io.Copy(w, resp.Body)
	resp.Body.Close()
}

func main() {
	flag.Parse()
	log.Println("Going to run chunker server on port: " + *port)
	log.Fatal(http.ListenAndServe(":"+*port, makeGzipHandler(ChunkerServer)))
}
