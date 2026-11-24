// Command ma is the headless manga side of InkShelfs, using the same engine and library
// as the desktop app:
//
//	go run ./cmd/ma sites
//	go run ./cmd/ma inspect <series-or-chapter-url>
//	go run ./cmd/ma pages <series-url> [chapter-number]
//	go run ./cmd/ma get <url> [-from 1] [-to 3] [-lib DIR]
//	go run ./cmd/ma dump <url>        print a page as the engine sees it
//	go run ./cmd/ma list [-lib DIR]
//	go run ./cmd/ma cbz <slug> <out.cbz> [-lib DIR]
//	go run ./cmd/ma test [-lib DIR]   discover + download chapter 1 of every site's example
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"novelarchiver/internal/manga"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	lib := fs.String("lib", manga.DefaultRoot(), "library folder")
	from := fs.Int("from", 0, "first chapter")
	to := fs.Int("to", 0, "last chapter")
	noBrowser := fs.Bool("no-browser", false, "don't open the site browser")
	var pos []string
	args := os.Args[2:]
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		pos, args = append(pos, args[0]), args[1:]
	}
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	status := func(m string) { fmt.Fprintln(os.Stderr, "…", m) }

	svc, err := manga.NewService(*lib, manga.ServiceOptions{NoBrowser: *noBrowser, OnStatus: status})
	check(err)
	defer svc.Shutdown()
	eng := svc.Engine()

	switch cmd {
	case "sites":
		for _, s := range svc.Sites() {
			fmt.Printf("%-14s %-26s %-12s %s\n", s.ID, s.Name, s.Status, s.Type)
		}
	case "inspect":
		need(pos, 1)
		s, err := eng.Discover(ctx, pos[0], status)
		check(err)
		first, last := s.Chapters[0], s.Chapters[len(s.Chapters)-1]
		out, _ := json.MarshalIndent(map[string]any{
			"site": s.SiteName, "type": s.Type, "title": s.Title, "author": s.Author, "status": s.Status,
			"cover": s.Cover, "url": s.URL, "description": clip(s.Description, 160), "chapters": len(s.Chapters),
			"first": first.Title + "  " + first.URL, "last": last.Title + "  " + last.URL,
			"inputChapter": s.InputChapter, "warnings": s.Warnings,
		}, "", "  ")
		fmt.Println(string(out))
	case "pages":
		need(pos, 1)
		s, err := eng.Discover(ctx, pos[0], status)
		check(err)
		n := 1
		if len(pos) > 1 {
			n, _ = strconv.Atoi(pos[1])
		}
		if n < 1 || n > len(s.Chapters) {
			check(fmt.Errorf("chapter %d out of range 1..%d", n, len(s.Chapters)))
		}
		imgs, ref, err := eng.Pages(ctx, s.SiteID, s.Vars, s.Chapters[n-1])
		check(err)
		fmt.Printf("%s — %d pages (referer %s)\n", s.Chapters[n-1].Title, len(imgs), ref)
		for i, u := range imgs {
			if i < 3 || i == len(imgs)-1 {
				fmt.Printf("  %3d %s\n", i+1, u)
			}
		}
	case "get":
		need(pos, 1)
		s, err := eng.Discover(ctx, pos[0], status)
		check(err)
		slug, err := svc.AddSeries(s, *from, *to)
		check(err)
		fmt.Println("queued", slug)
		waitIdle(ctx, svc)
		printBook(svc, slug)
	case "dump":
		// Save a page as the engine sees it (site browser for browser sites).
		need(pos, 1)
		pg, err := eng.Fetch(ctx, pos[0])
		check(err)
		os.Stdout.Write(pg.Body)
	case "list":
		items, err := svc.Library()
		check(err)
		for _, b := range items {
			fmt.Printf("%-40s %4d/%-4d failed=%d %7.1f MB  %s\n", b.Slug, b.Downloaded, b.Total, b.Failed, float64(b.Bytes)/(1<<20), b.URL)
		}
	case "cbz":
		need(pos, 2)
		res, err := svc.ExportCBZ(pos[0], pos[1])
		check(err)
		fmt.Printf("wrote %s (%d chapters, %d pages, %.1f MB)\n", res.Path, res.Chapters, res.Pages, float64(res.Bytes)/(1<<20))
	case "test":
		failed := 0
		for _, site := range svc.Sites() {
			if site.Example == "" {
				continue
			}
			start := time.Now()
			fmt.Printf("\n=== %s (%s)\n", site.Name, site.Example)
			s, err := eng.Discover(ctx, site.Example, nil)
			if err != nil {
				fmt.Println("  DISCOVER FAILED:", err)
				failed++
				continue
			}
			fmt.Printf("  %q by %q — %d chapters, cover=%v, first=%q last=%q\n", s.Title, s.Author, len(s.Chapters), s.Cover != "", s.Chapters[0].Title, s.Chapters[len(s.Chapters)-1].Title)
			slug, err := svc.AddSeries(s, 1, 1)
			if err != nil {
				fmt.Println("  ADD FAILED:", err)
				failed++
				continue
			}
			waitIdle(ctx, svc)
			d, _ := svc.Series(slug)
			c := d.Chapters[0]
			fmt.Printf("  chapter 1: state=%s pages=%d %.1f MB cover=%v  (%s)\n", c.State, c.Pages, float64(c.Bytes)/(1<<20), d.CoverFile != "", time.Since(start).Round(time.Second))
			if c.State != manga.StateOK {
				fmt.Println("  ERROR:", c.Error)
				failed++
			}
		}
		fmt.Printf("\n%d site(s) failed\n", failed)
	default:
		usage()
	}
}

func waitIdle(ctx context.Context, svc *manga.Service) {
	last := ""
	for {
		a := svc.Activity()
		if a.Current == nil && len(a.Queue) == 0 {
			return
		}
		if a.Current != nil {
			if m := fmt.Sprintf("  %s: %s", a.Current.Title, a.Current.Message); m != last {
				fmt.Fprintln(os.Stderr, m)
				last = m
			}
		}
		select {
		case <-ctx.Done():
			svc.CancelAll()
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func printBook(svc *manga.Service, slug string) {
	d, err := svc.Series(slug)
	check(err)
	ok, failed := 0, 0
	for _, c := range d.Chapters {
		switch c.State {
		case manga.StateOK:
			ok++
		case manga.StateFailed:
			failed++
			fmt.Printf("  failed %s: %s\n", c.Title, c.Error)
		}
	}
	fmt.Printf("%s: %d ok, %d failed, saved in %s\n", d.Title, ok, failed, svc.SeriesDir(slug))
}

func need(pos []string, n int) {
	if len(pos) < n {
		usage()
	}
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ma sites | inspect <url> | pages <url> [n] | get <url> [-from N] [-to N] | list | cbz <slug> <out.cbz> | test   [-lib DIR] [-no-browser]")
	os.Exit(2)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
