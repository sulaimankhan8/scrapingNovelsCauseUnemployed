// Package reader serves the offline reader:
//
//	/library/{slug}/          table of contents
//	/library/{slug}/{n}       chapter n
//	/library/{slug}/cover     cover image
//	/library/{slug}/img/{f}   chapter images
//
// Everything is rendered from the stored fragments with one template and
// system fonts, so it works with no internet connection at all.
package reader

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"novelarchiver/internal/library"
)

// Handler serves /library/... from a Store.
type Handler struct {
	Store *library.Store
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/library/")
	parts := strings.SplitN(rest, "/", 2)
	slug := parts[0]
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	b, err := h.Store.Load(slug)
	if err != nil {
		http.Error(w, "Novel not found in your library.", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	switch {
	case sub == "":
		h.toc(w, b)
	case sub == "cover":
		if b.CoverFile == "" {
			http.NotFound(w, r)
			return
		}
		p, err := h.Store.FilePath(slug, b.CoverFile)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, p)
	case strings.HasPrefix(sub, "img/"):
		p, err := h.Store.FilePath(slug, sub)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, p)
	default:
		n, err := strconv.Atoi(sub)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		h.chapter(w, b, n)
	}
}

type tocRow struct {
	Number int
	Title  string
	State  string
}

func (h *Handler) toc(w http.ResponseWriter, b *library.Book) {
	ok, failed, locked, pending := b.Counts()
	rows := make([]tocRow, len(b.Chapters))
	for i, c := range b.Chapters {
		rows[i] = tocRow{c.Number, c.Title, c.State}
	}
	render(w, tocTmpl, map[string]any{
		"Book": b, "Rows": rows, "OK": ok, "Failed": failed, "Locked": locked, "Pending": pending,
		"HasCover": b.CoverFile != "",
	})
}

func (h *Handler) chapter(w http.ResponseWriter, b *library.Book, n int) {
	idx := -1
	for i, c := range b.Chapters {
		if c.Number == n {
			idx = i
			break
		}
	}
	if idx < 0 {
		http.Error(w, "No such chapter.", http.StatusNotFound)
		return
	}
	c := b.Chapters[idx]
	body, err := h.Store.ReadChapter(b.Slug, n)
	missing := err != nil
	if !missing {
		_ = h.Store.SetReading(b.Slug, n) // the library shows "Continue · Ch. n"
	}
	prev, next := 0, 0
	if idx > 0 {
		prev = b.Chapters[idx-1].Number
	}
	if idx < len(b.Chapters)-1 {
		next = b.Chapters[idx+1].Number
	}
	render(w, chapterTmpl, map[string]any{
		"Book": b, "Ch": c, "Body": template.HTML(body), "Missing": missing,
		"Prev": prev, "Next": next, "Total": len(b.Chapters),
	})
}

func render(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		fmt.Fprintf(w, "<p>render error: %s</p>", template.HTMLEscapeString(err.Error()))
	}
}

// Body is already sanitised by internal/clean before it is stored.

const baseCSS = `
:root{--bg:#f7f3ec;--panel:#fffdf8;--ink:#1d1b18;--muted:#6b645a;--line:#e4dccd;--accent:#a8431f;
--sys-bg:#eef3f8;--sys-line:#7d9bb8;--sys-ink:#1f3347;--measure:40rem;--size:19px;--lh:1.75;
--font:"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Times New Roman",serif}
:root[data-theme=dark]{--bg:#121417;--panel:#191c20;--ink:#e7e2d9;--muted:#9a958c;--line:#2a2f35;--accent:#e0794f;
--sys-bg:#16222e;--sys-line:#3f6a92;--sys-ink:#cfe2f5}
:root[data-theme=sepia]{--bg:#efe4cc;--panel:#f6eedb;--ink:#3e3324;--muted:#7a6a52;--line:#dccdae;--accent:#8e4a1f;
--sys-bg:#e5e0cc;--sys-line:#8a7d58;--sys-ink:#3b3322}
:root[data-font=sans]{--font:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;--lh:1.65}
@media (prefers-color-scheme:dark){:root:not([data-theme]){--bg:#121417;--panel:#191c20;--ink:#e7e2d9;--muted:#9a958c;
--line:#2a2f35;--accent:#e0794f;--sys-bg:#16222e;--sys-line:#3f6a92;--sys-ink:#cfe2f5}}
*{box-sizing:border-box}html{-webkit-text-size-adjust:100%}
body{margin:0;background:var(--bg);color:var(--ink);font:var(--size)/var(--lh) var(--font)}
a{color:var(--accent)}
.bar{position:sticky;top:0;z-index:5;display:flex;gap:8px;align-items:center;justify-content:space-between;
padding:10px 16px;background:var(--panel);border-bottom:1px solid var(--line);font:14px/1.3 system-ui,sans-serif}
.bar a,.bar button{color:var(--ink);text-decoration:none;background:none;border:1px solid var(--line);
border-radius:8px;padding:7px 11px;font:inherit;cursor:pointer;min-height:36px}
.bar .title{overflow:hidden;white-space:nowrap;text-overflow:ellipsis;color:var(--muted);flex:1;min-width:0}
main{max-width:var(--measure);margin:0 auto;padding:24px 16px 96px}
`

var chapterTmpl = template.Must(template.New("ch").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Ch.Title}} · {{.Book.Title}}</title>
<style>` + baseCSS + `
article p{margin:0 0 1em}
article h1.ch-title{font-size:1.5em;line-height:1.25;margin:.4em 0 1.2em}
article img{max-width:100%;height:auto;display:block;margin:1em auto}
article hr{border:0;border-top:1px solid var(--line);margin:2em auto;width:40%}
article blockquote{margin:1em 0;padding-left:1em;border-left:3px solid var(--line);color:var(--muted)}
article .sys{background:var(--sys-bg);color:var(--sys-ink);border-left:3px solid var(--sys-line);
padding:.55em .8em;margin:0;font-family:ui-monospace,"Cascadia Mono",Consolas,monospace;font-size:.86em;line-height:1.55}
article .sys+.sys{border-top:0;padding-top:.15em}
article .sys:has(+ :not(.sys)),article .sys:last-child{margin-bottom:1em}
article table{border-collapse:collapse;margin:1em 0;width:100%;font-size:.9em;background:var(--sys-bg);color:var(--sys-ink)}
article td,article th{border:1px solid var(--sys-line);padding:.4em .6em;vertical-align:top}
.missing{padding:16px;border:1px dashed var(--line);border-radius:10px;color:var(--muted)}
.nav{display:flex;gap:12px;justify-content:space-between;margin-top:40px;font:15px system-ui,sans-serif}
.nav a,.nav span{flex:1;text-align:center;padding:14px;border:1px solid var(--line);border-radius:10px;text-decoration:none}
.nav span{opacity:.4}
</style></head><body>
<div class="bar">
 <a href="/#/novel/{{.Book.Slug}}" aria-label="Novel page and chapters">☰ Contents</a>
 <span class="title">{{.Book.Title}} — {{.Ch.Number}}/{{.Total}}</span>
 <button id="theme" type="button" aria-label="Change theme">Aa</button>
</div>
<main><article>
{{if .Missing}}<h1 class="ch-title">{{.Ch.Title}}</h1>
<p class="missing">This chapter isn't downloaded{{if .Ch.Error}} ({{.Ch.Error}}){{end}}. Press <b>Update</b> on the <a href="/#/novel/{{.Book.Slug}}">novel page</a> to fetch it.</p>
{{else}}{{.Body}}{{end}}
</article>
<nav class="nav">
 {{if .Prev}}<a href="{{.Prev}}" rel="prev">← Previous</a>{{else}}<span>← Previous</span>{{end}}
 {{if .Next}}<a href="{{.Next}}" rel="next">Next →</a>{{else}}<span>Next →</span>{{end}}
</nav></main>
<script>
(function(){
 var KEY='na-reader', s={};
 try{s=JSON.parse(localStorage.getItem(KEY)||'{}')}catch(e){}
 var root=document.documentElement;
 var app=''; try{app=localStorage.getItem('na-theme')||''}catch(e){}
 function apply(){ if(s.theme)root.dataset.theme=s.theme; else if(app&&app!=='system')root.dataset.theme=app; if(s.font)root.dataset.font=s.font;
   if(s.size)root.style.setProperty('--size',s.size+'px'); }
 function save(){ try{localStorage.setItem(KEY,JSON.stringify(s))}catch(e){} }
 apply();
 try{localStorage.setItem('na-last-{{.Book.Slug}}','{{.Ch.Number}}')}catch(e){}
 var themes=[undefined,'dark','sepia'];
 document.getElementById('theme').onclick=function(){
   var i=themes.indexOf(s.theme); s.theme=themes[(i+1)%themes.length]||''; if(!s.theme){delete s.theme;delete root.dataset.theme}
   apply(); save(); };
 document.addEventListener('keydown',function(e){
   if(e.target.tagName==='INPUT')return;
   if(e.key==='ArrowRight'){var n=document.querySelector('a[rel=next]'); if(n)location.href=n.href}
   if(e.key==='ArrowLeft'){var p=document.querySelector('a[rel=prev]'); if(p)location.href=p.href}
   if(e.key==='+'||e.key==='='){s.size=Math.min(30,(s.size||19)+1);apply();save()}
   if(e.key==='-'){s.size=Math.max(13,(s.size||19)-1);apply();save()}
   if(e.key==='f'){s.font=s.font==='sans'?'':'sans'; if(!s.font)delete root.dataset.font; apply();save()}
   if(e.key==='h'||e.key==='H'){location.href='/#/novel/{{.Book.Slug}}'}
 });
})();
</script></body></html>`))

var tocTmpl = template.Must(template.New("toc").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Book.Title}}</title>
<style>` + baseCSS + `
main{max-width:52rem;font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif}
.head{display:flex;gap:20px;align-items:flex-start;flex-wrap:wrap}
.head img{width:120px;aspect-ratio:2/3;object-fit:cover;border-radius:6px;border:1px solid var(--line)}
.head h1{font:600 1.7em/1.2 var(--font);margin:0 0 6px}
.meta{color:var(--muted);margin:0 0 10px}
.desc{white-space:pre-line;color:var(--ink);max-height:12em;overflow:auto;margin:0}
.stats{display:flex;gap:14px;flex-wrap:wrap;color:var(--muted);font-size:14px;margin:18px 0}
.cta{display:inline-block;margin:6px 0 18px;padding:10px 16px;border-radius:10px;background:var(--accent);color:#fff;text-decoration:none}
input{width:100%;padding:10px 12px;border:1px solid var(--line);border-radius:10px;background:var(--panel);color:var(--ink);font:inherit;margin-bottom:12px}
ol{list-style:none;padding:0;margin:0;border-top:1px solid var(--line)}
li a,li span{display:flex;gap:12px;padding:10px 4px;border-bottom:1px solid var(--line);text-decoration:none;color:var(--ink)}
li .n{color:var(--muted);min-width:3.5em;font-variant-numeric:tabular-nums}
li.failed span,li.pending span,li.locked span{color:var(--muted)}
li .tag{margin-left:auto;font-size:12px;color:var(--muted)}
</style></head><body>
<div class="bar"><a href="/" aria-label="Back to library">← Library</a><span class="title">{{.Book.SiteName}}</span><span></span></div>
<main>
<div class="head">
 {{if .HasCover}}<img src="cover" alt="">{{end}}
 <div style="flex:1;min-width:240px">
  <h1>{{.Book.Title}}</h1>
  <p class="meta">{{if .Book.Author}}by {{.Book.Author}} · {{end}}{{len .Rows}} chapters</p>
  <p class="desc">{{.Book.Description}}</p>
 </div>
</div>
<div class="stats"><span>✓ {{.OK}} saved</span>{{if .Pending}}<span>{{.Pending}} not downloaded</span>{{end}}{{if .Failed}}<span>{{.Failed}} failed</span>{{end}}{{if .Locked}}<span>{{.Locked}} paid/locked</span>{{end}}</div>
<a class="cta" id="continue" href="1">Start reading</a>
<input id="q" type="search" placeholder="Find a chapter by number or title" aria-label="Find a chapter">
<ol id="list">{{range .Rows}}<li class="{{if eq .State "ok"}}ok{{else if .State}}{{.State}}{{else}}pending{{end}}" data-t="{{.Number}} {{.Title}}">
{{if eq .State "ok"}}<a href="{{.Number}}"><span class="n">{{.Number}}</span>{{.Title}}</a>{{else}}<span><span class="n">{{.Number}}</span>{{.Title}}<span class="tag">{{if eq .State "locked"}}paid{{else if eq .State "failed"}}failed{{else}}not saved{{end}}</span></span>{{end}}
</li>{{end}}</ol>
</main>
<script>
(function(){
 try{var s=JSON.parse(localStorage.getItem('na-reader')||'{}'),app=localStorage.getItem('na-theme')||''; if(s.theme)document.documentElement.dataset.theme=s.theme; else if(app&&app!=='system')document.documentElement.dataset.theme=app;}catch(e){}
 try{var last=localStorage.getItem('na-last-{{.Book.Slug}}'); if(last){var c=document.getElementById('continue'); c.href=last; c.textContent='Continue · chapter '+last;}}catch(e){}
 var q=document.getElementById('q'), items=[].slice.call(document.querySelectorAll('#list li'));
 q.addEventListener('input',function(){var v=q.value.toLowerCase().trim();
   items.forEach(function(li){li.style.display=!v||li.dataset.t.toLowerCase().indexOf(v)>=0?'':'none'})});
})();
</script></body></html>`))
