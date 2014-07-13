package main

import "io"
import "log"
import "flag"
import "strings"
import "net/http"
import "io/ioutil"
import "compress/gzip"

var (
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
	io.Copy(w, resp.Body)
	resp.Body.Close()	
	if resp.StatusCode != 200 {
		return
	}
    lastid := "0"
	var content []byte
	for {
		resp, _ = http_client.Do(getRequest(url + "&lastid=" + lastid))
		if resp.StatusCode != 200 {
			return
		}
		content, _ = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if string(content) == "" {
			break
		}
		io.WriteString(w, string(content))
		lastid = resp.Header.Get("LASTID")
	}
	resp, _ = http_client.Do(getRequest(url + "&footer=1"))
	if resp.StatusCode != 200 {
		return
	}
	io.Copy(w, resp.Body)
	resp.Body.Close()
}

func main() {
    port := flag.String("port", "8080", "port to run the server on")
    flag.Parse()
    log.Println("Going to run chunker server on port: " + *port)
	log.Fatal(http.ListenAndServe(":" + *port, makeGzipHandler(ChunkerServer)))
}
