package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"timedog/internal/api"
)

//go:embed all:web/dist
var embeddedDist embed.FS // relative to this package directory

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	staticDir := flag.String("static", "", "optional directory with built frontend (overrides embed when set)")
	flag.Parse()

	apiMux := api.NewAPIRouter()
	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", apiMux))

	var static http.Handler
	if *staticDir != "" {
		static = spaStatic(os.DirFS(*staticDir))
	} else {
		dist, err := fs.Sub(embeddedDist, "web/dist")
		if err != nil {
			log.Printf("embed web/dist: %v (API only)", err)
		} else {
			static = spaStatic(dist)
		}
	}
	if static != nil {
		root.Handle("/", static)
	}

	log.Printf("timedog-server listening on http://%s", *addr)
	if err := http.ListenAndServe(*addr, root); err != nil {
		log.Fatal(err)
	}
}

// spaStatic serves the Vite build: assets by path, client routes → index.html.
// Использует ServeFileFS вместо FileServer + подмены URL — иначе возможен 301 Location: ./ и пустая страница в браузере.
func spaStatic(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(r.URL.Path, "/")
		if upath != "" {
			upath = path.Clean(upath)
		}
		if upath == "." || upath == "" {
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}

		fi, err := fs.Stat(fsys, upath)
		if err != nil {
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}
		if fi.IsDir() {
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}
		http.ServeFileFS(w, r, fsys, upath)
	})
}
