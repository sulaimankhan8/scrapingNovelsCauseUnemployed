// Command na is a headless NovelArchiver: it uses the same engine and
// library as the desktop app.
//
//	go run ./cmd/na inspect <novel-or-chapter-url>
//	go run ./cmd/na get <url> [-from 1] [-to 10] [-lib DIR]
//	go run ./cmd/na list [-lib DIR]
//	go run ./cmd/na test [-site ID] [-optin]   every site: discover, chapter 1, search
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"novelarchiver/internal/archiver"
	"novelarchiver/internal/browser"
	"novelarchiver/internal/fetch"
	"novelarchiver/internal/library"
	"novelarchiver/internal/sites"
	"novelarchiver/internal/source"
	"novelarchiver/internal/translate"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	lib := fs.String("lib", library.DefaultRoot(), "library folder")
	from := fs.Int("from", 0, "first chapter")
	to := fs.Int("to", 0, "last chapter")
	workers := fs.Int("workers", 3, "parallel downloads")
	noImages := fs.Bool("no-images", false, "skip chapter images")
	only := fs.String("site", "", "test: only this site id")
	optIn := fs.Bool("optin", false, "test: also run searches that usually ask for a human check")
	args := os.Args[2:]
	var url string
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		url, args = args[0], args[1:]
	}
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	store, err := library.Open(*lib)
	check(err)
	reg, err := sites.Load("")
	check(err)
	httpF := fetch.NewHTTP(fetch.Options{})
	status := func(m string) { fmt.Fprintln(os.Stderr, "…", m) }
	eng := &source.Engine{Registry: reg, HTTP: httpF}
	if win, err := browser.New(browser.Options{
		DataPath: filepath.Join(filepath.Dir(store.Root), "SiteBrowser"),
		OnStatus: status,
	}); err == nil {
		eng.Browser = win
		defer win.Close()
	}

	switch cmd {
	case "inspect":
		need(url)
		n, err := eng.Discover(ctx, url, status)
		check(err)
		first, last := "", ""
		if len(n.Chapters) > 0 {
			first, last = n.Chapters[0].Title, n.Chapters[len(n.Chapters)-1].Title
		}
		out, _ := json.MarshalIndent(map[string]any{
			"site": n.SiteName, "title": n.Title, "author": n.Author, "cover": n.Cover, "url": n.URL,
			"chapters": len(n.Chapters), "first": first, "last": last, "tocMode": n.TOCMode,
			"firstChapterUrl": n.FirstChapterURL, "inputKind": n.InputKind, "inputChapter": n.InputChapter,
			"warnings": n.Warnings,
		}, "", "  ")
		fmt.Println(string(out))
	case "get":
		need(url)
		n, err := eng.Discover(ctx, url, status)
		check(err)
		a := &archiver.Archiver{Engine: eng, Store: store, Files: httpF, Translator: translate.NewGoogle()}
		b, err := a.BookFromNovel(n)
		check(err)
		err = a.Run(ctx, b, n, archiver.Options{From: *from, To: *to, Concurrency: *workers, SkipImages: *noImages},
			func(ev string, data any) {
				switch d := data.(type) {
				case archiver.Progress:
					fmt.Printf("[%3d%%] ch %d %-6s %s\n", d.Percentage, d.ChapterNum, d.Status, d.ChapterTitle)
				default:
					j, _ := json.Marshal(d)
					fmt.Printf("%s %s\n", ev, j)
				}
			})
		check(err)
		ok, failed, locked, _ := b.Counts()
		fmt.Printf("saved to %s/%s  ok=%d failed=%d locked=%d\n", store.Root, b.Slug, ok, failed, locked)
		for _, c := range b.Chapters {
			if c.State == library.StateFailed {
				fmt.Printf("  failed ch %d: %s\n", c.Number, c.Error)
			}
		}
	case "dump":
		// Save a page exactly as the engine sees it (browser for browser-only
		// sites), for writing or fixing site profiles.
		need(url)
		var f fetch.Fetcher = httpF
		if m, err := reg.Match(url); err == nil && m.Profile != nil && m.Profile.FetchMode != "http" && eng.Browser != nil {
			f = eng.Browser
		}
		page, err := f.Get(ctx, url, "")
		check(err)
		os.Stdout.Write(page.Body)
	case "test":
		if testSites(ctx, eng, *only, *optIn) > 0 {
			os.Exit(1)
		}
	case "list":
		books, err := store.List()
		check(err)
		for _, b := range books {
			ok, failed, _, _ := b.Counts()
			fmt.Printf("%-40s %4d/%-4d failed=%d  %.1f MB  %s\n", b.Slug, ok, len(b.Chapters), failed, float64(store.Size(b.Slug))/(1<<20), b.URL)
		}
	default:
		usage()
	}
}

func need(url string) {
	if url == "" {
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: na inspect <url> | na dump <url> | na get <url> [-from N] [-to N] [-lib DIR] | na list | na test [-site ID] [-optin]")
	os.Exit(2)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
