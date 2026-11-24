// Command androidsim runs the Android app's UI in a desktop browser, against
// the real Android Go core (package mobile), with no emulator or phone. It
// plays the part of the Java shell: window.NovelBridge calls, engine events,
// /library/ and /mangalib/ files, and the exports Java normally copies into
// Downloads (written to a temp folder here). Open it at phone size.
//
//	go run ./cmd/androidsim -root <folder>     then open http://localhost:34145
//
// <folder>/library holds novels and <folder>/manga/library holds manga,
// exactly as on the phone. There is no site window, so sites that need a
// "verify you are human" check can't be tested here.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"novelarchiver/mobile"
)

// bridgeJS stands in for MainActivity's addJavascriptInterface. It must
// load before android-shim.js, which looks for window.NovelBridge.
const bridgeJS = `window.NovelBridge = {
  call: function (id, method, args) {
    fetch('/__call', { method: 'POST', body: JSON.stringify({ method: method, args: args }) })
      .then(function (r) { return r.json(); })
      .then(function (o) { window.__naResolve && window.__naResolve(id, o.ok, o.payload); })
      .catch(function (e) { window.__naResolve && window.__naResolve(id, false, String(e)); });
  }
};
(function () {
  var es = new EventSource('/__events');
  es.onmessage = function (e) {
    var m = JSON.parse(e.data);
    window.__naEmit && window.__naEmit(m.event, m.payload);
  };
})();
window.__simAndroid = true;
`

type hub struct {
	mu   sync.Mutex
	subs map[chan string]bool
}

func (h *hub) Emit(event, payload string) {
	b, _ := json.Marshal(map[string]string{"event": event, "payload": payload})
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		select {
		case c <- string(b):
		default: // a slow page drops events, like a busy WebView would
		}
	}
}

func main() {
	root := flag.String("root", "", "folder holding library/ and manga/library/ (default: a temp folder)")
	port := flag.Int("port", 34145, "port to serve on")
	front := flag.String("frontend", "frontend", "the shared frontend folder")
	flag.Parse()
	if *root == "" {
		d, err := os.MkdirTemp("", "androidsim-")
		if err != nil {
			log.Fatal(err)
		}
		*root = d
	}
	exports := filepath.Join(*root, "Downloads")
	_ = os.MkdirAll(exports, 0o755)

	h := &hub{subs: map[chan string]bool{}}
	core, err := mobile.New(filepath.Join(*root, "library"), h, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer core.Shutdown()

	mux := http.NewServeMux()
	mux.HandleFunc("/__sim.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		io.WriteString(w, bridgeJS)
	})
	mux.HandleFunc("/__events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		c := make(chan string, 256)
		h.mu.Lock()
		h.subs[c] = true
		h.mu.Unlock()
		defer func() { h.mu.Lock(); delete(h.subs, c); h.mu.Unlock() }()
		fl, _ := w.(http.Flusher)
		for {
			select {
			case m := <-c:
				fmt.Fprintf(w, "data: %s\n\n", m)
				if fl != nil {
					fl.Flush()
				}
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("/__call", func(w http.ResponseWriter, r *http.Request) {
		// args arrive already JSON-encoded, as NovelBridge.call receives them.
		var req struct {
			Method string `json:"method"`
			Args   string `json:"args"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		payload, err := javaCall(core, req.Method, req.Args, exports)
		ok := err == nil
		if err != nil {
			payload = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": ok, "payload": payload})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/library/") || strings.HasPrefix(p, "/mangalib/") {
			res := core.Serve(p)
			w.Header().Set("Content-Type", res.MimeType)
			w.WriteHeader(int(res.Status))
			w.Write(res.Body)
			return
		}
		if p == "/" || p == "/index.html" {
			b, err := os.ReadFile(filepath.Join(*front, "index.html"))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			html := strings.Replace(string(b), `<script src="android-shim.js"></script>`,
				`<script src="/__sim.js"></script><script src="android-shim.js"></script>`, 1)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, html)
			return
		}
		http.ServeFile(w, r, filepath.Join(*front, filepath.FromSlash(strings.TrimPrefix(p, "/"))))
	})
	addr := fmt.Sprintf("localhost:%d", *port)
	log.Printf("Android UI on http://%s  (library: %s, exports: %s)", addr, *root, exports)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// javaCall mirrors NovelBridge.call: the methods Java handles itself, then
// everything else straight to the Go core.
func javaCall(core *mobile.Core, method, args, exports string) (string, error) {
	first := func() string {
		var a []string
		_ = json.Unmarshal([]byte(args), &a)
		if len(a) == 0 {
			return ""
		}
		return a[0]
	}
	switch method {
	case "ExportEPUB":
		return withPath(core.ExportEPUB(first(), exports))
	case "Manga.ExportCBZ":
		return withPath(core.ExportCBZ(first(), exports))
	case "BackupLibrary":
		return withPath(core.Backup(exports))
	case "ChooseBackup":
		return "", nil // the file picker is the phone's; treat as cancelled
	case "Notify":
		log.Printf("notification: %s", args)
		return "", nil
	}
	return core.Call(method, args)
}

// withPath reports "Downloads" as the place, as the phone does.
func withPath(js string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	var m map[string]any
	if json.Unmarshal([]byte(js), &m) == nil {
		log.Printf("export written: %v", m["path"])
		m["path"] = "Downloads"
		b, _ := json.Marshal(m)
		return string(b), nil
	}
	return js, nil
}
