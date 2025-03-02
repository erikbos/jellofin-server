package notflix

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// mosyly taken from https://gist.github.com/yowu/f7dc34bd4736a65ff28d
// Thanks yowu.

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func buildUrl(server string, path string) (*url.URL, error) {
	elems := strings.Split(path, "/")
	for i, s := range elems {
		elems[i] = url.PathEscape(s)
	}
	u := server + strings.Join(elems, "/")
	return url.ParseRequestURI(u)
}

func (n *Notflix) hlsHandler(w http.ResponseWriter, r *http.Request) bool {
	vars := mux.Vars(r)
	path, pathOk := vars["path"]
	if !pathOk {
		return false
	}
	source, sourceOk := vars["source"]
	if !sourceOk {
		return false
	}
	if !strings.Contains(path, ".mp4/") {
		return false
	}
	c := n.collections.GetCollection(source)
	if c == nil {
		return false
	}
	hlsServer := c.GetHlsServer()
	// hlsServer := n.collections.GetHlsServer(source)
	// if hlsServer == "" {
	// 	return false
	// }
	url, err := buildUrl(hlsServer, path)
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return true
	}

	// will be added by remote server.
	w.Header().Del("access-control-allow-origin")
	w.Header().Del("access-control-allow-methods")

	delHopHeaders(r.Header)

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		appendHostToXForwardHeader(r.Header, clientIP)
	}
	r.RequestURI = ""
	r.URL = url

	fmt.Printf("connecting to %v\n", url)

	resp, err := netClient.Do(r)
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return true
	}
	defer resp.Body.Close()

	fmt.Printf("%v connected to %v\n", resp.Status, url)

	delHopHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	return true
}
