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
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"novelarchiver/internal/library"
)

// Handler serves /library/... from a Store.
type Handler struct {
	Store *library.Store
	// Translate puts one saved chapter into English now (nil: not offered).
	// An untranslated chapter is translated as soon as it's opened.
	Translate func(ctx context.Context, slug string, n int) error
	// Paused is how long Google has asked the app to wait (nil: never).
	Paused func() time.Duration
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
		trErr := ""
		ask := r.URL.Query().Get("translate")
		if (ask == "now" || ask == "auto") && h.Translate != nil && b.Foreign() {
			if err := h.Translate(r.Context(), b.Slug, n); err != nil {
				trErr = err.Error()
			}
			if nb, err := h.Store.Load(slug); err == nil {
				b = nb
			}
		}
		h.chapter(w, b, n, r.URL.Query().Get("text") == "original", trErr, h.Translate != nil, ask == "")
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

// chapter renders one chapter. For a translated novel it shows the
// English copy unless original is set (or there is no English copy yet).
func (h *Handler) chapter(w http.ResponseWriter, b *library.Book, n int, original bool, trErr string, canTranslate, firstLook bool) {
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
	foreign := b.Foreign()
	showEN, untranslated := false, false
	if foreign && !missing {
		if en, err := h.Store.ReadTranslation(b.Slug, n); err == nil {
			if !original {
				body, showEN = en, true
			}
		} else {
			untranslated = true
		}
	}
	// Opening an untranslated chapter translates it (the page reloads
	// with the English) unless Google has asked for a break.
	var paused time.Duration
	if h.Paused != nil {
		paused = h.Paused()
	}
	auto := untranslated && !original && canTranslate && firstLook && trErr == "" && paused == 0
	pausedMin := 0
	if paused > 0 && trErr == "" {
		pausedMin = int(paused.Minutes() + 0.99)
	}
	htmlLang := "en"
	if foreign && !showEN {
		htmlLang = b.Language
	}
	query := ""
	if foreign && original {
		query = "?text=original"
	}
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
		"Foreign": foreign, "ShowEN": showEN, "Untranslated": untranslated && !original,
		"Lang": htmlLang, "Query": template.URL(query), "OrigLabel": origLabel(b.Language),
		"CanTranslate": canTranslate && paused == 0, "TranslateErr": trErr, "Auto": auto, "PausedMin": pausedMin,
	})
}

// origLabel is "original text" in the novel's own language, for the switch.
func origLabel(code string) string {
	switch code {
	case "zh":
		return "原文"
	case "ja":
		return "原文"
	case "ko":
		return "원문"
	}
	return "Original"
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
--font:"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Noto Serif",serif}
:root[data-theme=dark]{--bg:#121417;--panel:#191c20;--ink:#e7e2d9;--muted:#9a958c;--line:#2a2f35;--accent:#e0794f;
--sys-bg:#16222e;--sys-line:#3f6a92;--sys-ink:#cfe2f5}
:root[data-theme=sepia]{--bg:#efe4cc;--panel:#f6eedb;--ink:#3e3324;--muted:#7a6a52;--line:#dccdae;--accent:#8e4a1f;
--sys-bg:#e5e0cc;--sys-line:#8a7d58;--sys-ink:#3b3322}
:root[data-font=sans]{--font:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;--lh:1.65}
:root[data-theme=light]{color-scheme:light}:root[data-theme=dark]{color-scheme:dark}
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
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Ch.Title}} · {{.Book.Title}}</title>
<script>
(function(){try{var s=JSON.parse(localStorage.getItem('na-reader')||'{}'),app=localStorage.getItem('na-theme')||'',r=document.documentElement;
if(s.theme)r.dataset.theme=s.theme;else if(app&&app!=='system')r.dataset.theme=app;
if(s.font==='sans')r.dataset.font='sans';if(s.align==='justify')r.dataset.align='justify';
if(s.size)r.style.setProperty('--size',s.size+'px');if(s.lh)r.style.setProperty('--lh',s.lh);if(s.width)r.style.setProperty('--measure',s.width+'rem');}catch(e){}})();
{{if .Foreign}}// Remembered EN / original choice for translated novels.
(function(){try{var want=localStorage.getItem('na-text')==='original', has=location.search.indexOf('text=original')>=0;
if(want&&!has)location.replace('?text=original');
{{if .Auto}}else if(!want)location.replace('?translate=auto');{{end}}
if(/[?&]translate=/.test(location.search))history.replaceState(null,'',location.pathname);}catch(e){}})();{{end}}
</script>
<style>` + baseCSS + `
article p{margin:0 0 1em}
.lang{display:inline-flex;gap:2px;padding:2px;border:1px solid var(--line);border-radius:9px}
.bar .lang a{border:0;min-height:30px;padding:5px 9px;border-radius:7px;font-size:13px}
.bar .lang a[aria-current=true]{background:var(--ink);color:var(--bg);font-weight:600}
.note{padding:10px 12px;border-radius:10px;background:var(--sys-bg);color:var(--sys-ink);font:14px/1.45 system-ui,sans-serif;margin:0 0 1.2em}
.tr-now{display:inline-block;margin-left:6px;padding:6px 12px;border-radius:8px;background:var(--ink);color:var(--bg);font-weight:600;text-decoration:none;white-space:nowrap}
.tr-now[aria-disabled=true]{opacity:.6;pointer-events:none}
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
:root[data-align=justify] article p{text-align:justify;hyphens:auto;-webkit-hyphens:auto}
#theme{font:600 16px/1 "Iowan Old Style",Georgia,serif;min-width:44px}
#theme[aria-expanded=true]{background:var(--bg)}
.aa{position:fixed;top:57px;left:0;right:0;max-height:calc(100vh - 57px);overflow-y:auto;overscroll-behavior:contain;z-index:4;background:var(--panel);border-bottom:1px solid var(--line);font:14px/1.3 system-ui,-apple-system,"Segoe UI",sans-serif;
box-shadow:0 12px 24px -16px rgba(0,0,0,.35)}
.aa[hidden]{display:none}
.aa-in{max-width:34rem;margin:0 auto;padding:6px 16px 14px}
.aa-row{display:flex;align-items:center;justify-content:space-between;gap:8px 14px;flex-wrap:wrap;padding:9px 0;border-bottom:1px solid var(--line)}
.aa-row:last-child{border-bottom:0}
.aa-row>b{font-weight:500}
.aa-seg{display:inline-flex;gap:2px;padding:3px;border-radius:10px;background:var(--bg);border:1px solid var(--line)}
.aa-seg button,.aa-step button{font:inherit;color:var(--ink);background:none;border:0;border-radius:7px;padding:7px 12px;min-height:36px;cursor:pointer}
.aa-seg button[aria-checked=true]{background:var(--ink);color:var(--bg);font-weight:600}
.aa-step{display:inline-flex;align-items:center;gap:6px}
.aa-step button{border:1px solid var(--line);background:var(--bg);min-width:48px}
.aa-step button:disabled{opacity:.4;cursor:default}
.aa-step output{min-width:4.5ch;text-align:center;font-variant-numeric:tabular-nums;color:var(--muted)}
@media (max-width:640px){.aa-wide{display:none}.aa-row{flex-direction:column;align-items:stretch}.aa-seg{display:flex}.aa-seg button{flex:1}.aa-step{justify-content:space-between}}
</style></head><body>
<div class="bar">
 <a href="/#/novel/{{.Book.Slug}}" aria-label="Novel page and chapters">☰ Contents</a>
 <span class="title">{{.Book.Title}} — {{.Ch.Number}}/{{.Total}}</span>
 {{if .Foreign}}<span class="lang" role="group" aria-label="Text language"><a href="?" data-text="" aria-current="{{if .ShowEN}}true{{else}}false{{end}}" lang="en">EN</a><a href="?text=original" data-text="original" aria-current="{{if .ShowEN}}false{{else}}true{{end}}" lang="{{.Book.Language}}">{{.OrigLabel}}</a></span>{{end}}
 <button id="theme" type="button" aria-label="Reading settings" aria-expanded="false" aria-controls="aa">Aa</button>
</div>
<div class="aa" id="aa" role="dialog" aria-label="Reading settings" hidden><div class="aa-in" id="aaIn"></div></div>
<main><article>
{{if .Missing}}<h1 class="ch-title">{{.Ch.Title}}</h1>
<p class="missing">This chapter isn't downloaded{{if .Ch.Error}} ({{.Ch.Error}}){{end}}. Press <b>Update</b> on the <a href="/#/novel/{{.Book.Slug}}">novel page</a> to fetch it.</p>
{{else}}{{if .Untranslated}}<p class="note" lang="en">{{if .Auto}}Translating this chapter…{{else if .TranslateErr}}Couldn't translate it: {{.TranslateErr}}{{else if .PausedMin}}Google Translate asked the app to take a break, so this chapter is shown as it was written. It's translated by itself in about {{.PausedMin}} min.{{else}}This chapter isn't translated yet, so it's shown as it was written.{{end}}{{if and .CanTranslate (not .Auto)}} <a class="tr-now" href="?translate=now" onclick="this.textContent='Translating…';this.setAttribute('aria-disabled','true')">Translate now</a>{{end}}</p>{{end}}{{.Body}}{{end}}
</article>
<nav class="nav">
 {{if .Prev}}<a href="{{.Prev}}{{.Query}}" rel="prev">← Previous</a>{{else}}<span>← Previous</span>{{end}}
 {{if .Next}}<a href="{{.Next}}{{.Query}}" rel="next">Next →</a>{{else}}<span>Next →</span>{{end}}
</nav></main>
<script>
(function(){
 document.querySelectorAll('.lang a').forEach(function(a){a.addEventListener('click',function(){try{if(a.dataset.text)localStorage.setItem('na-text',a.dataset.text);else localStorage.removeItem('na-text')}catch(e){}})});
 var KEY='na-reader', s={};
 try{s=JSON.parse(localStorage.getItem(KEY)||'{}')}catch(e){}
 var root=document.documentElement;
 var app=''; try{app=localStorage.getItem('na-theme')||''}catch(e){}
 function set(k,v){ if(v===undefined||v===''){delete root.dataset[k]}else{root.dataset[k]=v} }
 function apply(){
   set('theme', s.theme || (app && app!=='system' ? app : ''));
   set('font', s.font==='sans' ? 'sans' : ''); set('align', s.align==='justify' ? 'justify' : '');
   root.style.setProperty('--size',(s.size||19)+'px');
   if(s.lh)root.style.setProperty('--lh',s.lh); else root.style.removeProperty('--lh');
   root.style.setProperty('--measure',(s.width||40)+'rem'); }
 function save(){ try{localStorage.setItem(KEY,JSON.stringify(s))}catch(e){} }
 apply();
 try{localStorage.setItem('na-last-{{.Book.Slug}}','{{.Ch.Number}}')}catch(e){}
 // Aa: the same reading settings as the app (Settings > Reading).
 var btn=document.getElementById('theme'), aa=document.getElementById('aa'), aaIn=document.getElementById('aaIn');
 function seg(k,opts,label,cur){ return '<div class="aa-seg" role="radiogroup" aria-label="'+label+'">'+opts.map(function(o){
   return '<button type="button" role="radio" data-k="'+k+'" data-v="'+o[0]+'" aria-checked="'+(String(cur)===String(o[0]))+'">'+o[1]+'</button>'}).join('')+'</div>'; }
 function row(label,ctl,cls){ return '<div class="aa-row'+(cls||'')+'"><b>'+label+'</b>'+ctl+'</div>'; }
 function paint(){ var z=s.size||19;
   aaIn.innerHTML=row('Theme',seg('theme',[['','App'],['light','Light'],['sepia','Sepia'],['dark','Dark']],'Theme',s.theme||''))
   +row('Font',seg('font',[['serif','Serif'],['sans','Sans']],'Font',s.font||'serif'))
   +row('Text size','<div class="aa-step"><button type="button" data-step="-1" aria-label="Smaller text"'+(z<=13?' disabled':'')+'>A−</button><output aria-live="polite">'+z+' px</output><button type="button" data-step="1" aria-label="Larger text"'+(z>=30?' disabled':'')+'>A+</button></div>')
   +row('Line spacing',seg('lh',[['1.5','Tight'],['1.75','Normal'],['2','Loose']],'Line spacing',s.lh||'1.75'))
   +row('Page width',seg('width',[['34','Narrow'],['40','Normal'],['48','Wide']],'Page width',s.width||'40'),' aa-wide')
   +row('Alignment',seg('align',[['left','Left'],['justify','Justified']],'Alignment',s.align||'left')); }
 function open(v){ aa.hidden=!v; btn.setAttribute('aria-expanded',String(v)); if(v)paint(); }
 btn.onclick=function(e){ e.stopPropagation(); open(aa.hidden); };
 aa.addEventListener('click',function(e){
   var b=e.target.closest('button'); if(!b||b.disabled)return;
   // Keep the line you were reading in place while the text reflows.
   var y=window.scrollY, h=document.documentElement.scrollHeight;
   if(b.dataset.step){ s.size=Math.max(13,Math.min(30,(s.size||19)+Number(b.dataset.step))); if(s.size===19)delete s.size; }
   else { var k=b.dataset.k, v=b.dataset.v;
     if(k==='lh'||k==='width')v=Number(v);
     if(v===''||(k==='font'&&v==='serif')||(k==='align'&&v==='left')||(k==='lh'&&v===1.75)||(k==='width'&&v===40))delete s[k]; else s[k]=v; }
   apply(); save(); paint();
   var h2=document.documentElement.scrollHeight; if(h&&y>0)window.scrollTo(0,y*h2/h);
 });
 document.addEventListener('click',function(e){ if(!aa.hidden&&!aa.contains(e.target))open(false); });
 document.addEventListener('keydown',function(e){
   if(e.target.tagName==='INPUT')return;
   if(e.key==='ArrowRight'){var n=document.querySelector('a[rel=next]'); if(n)location.href=n.href}
   if(e.key==='ArrowLeft'){var p=document.querySelector('a[rel=prev]'); if(p)location.href=p.href}
   if(e.key==='+'||e.key==='='){s.size=Math.min(30,(s.size||19)+1);apply();save()}
   if(e.key==='-'){s.size=Math.max(13,(s.size||19)-1);apply();save()}
   if(e.key==='f'){s.font=s.font==='sans'?'':'sans'; if(!s.font)delete root.dataset.font; apply();save()}
   if(e.key==='h'||e.key==='H'){location.href='/#/novel/{{.Book.Slug}}'}
   if(e.key==='Escape'&&!aa.hidden)open(false);
   if(!aa.hidden)paint();
 });
})();
</script></body></html>`))

var tocTmpl = template.Must(template.New("toc").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Book.Title}}</title>
<script>
(function(){try{var s=JSON.parse(localStorage.getItem('na-reader')||'{}'),app=localStorage.getItem('na-theme')||'',r=document.documentElement;
if(s.theme)r.dataset.theme=s.theme;else if(app&&app!=='system')r.dataset.theme=app;
if(s.font==='sans')r.dataset.font='sans';if(s.align==='justify')r.dataset.align='justify';
if(s.size)r.style.setProperty('--size',s.size+'px');if(s.lh)r.style.setProperty('--lh',s.lh);if(s.width)r.style.setProperty('--measure',s.width+'rem');}catch(e){}})();
</script>
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
.pager{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:12px 0}
.pager[hidden]{display:none}
.pager button,.pager select{font:inherit;font-size:14px;color:var(--ink);background:var(--panel);border:1px solid var(--line);border-radius:10px;min-height:40px;padding:0 12px}
.pager button:disabled{opacity:.4}
.pager span{display:flex;align-items:center;gap:8px;color:var(--muted);font-size:13px;white-space:nowrap}
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
<nav class="pager" id="pagerTop" aria-label="Chapter pages" hidden></nav>
<ol id="list">{{range .Rows}}<li class="{{if eq .State "ok"}}ok{{else if .State}}{{.State}}{{else}}pending{{end}}" data-t="{{.Number}} {{.Title}}">
{{if eq .State "ok"}}<a href="{{.Number}}"><span class="n">{{.Number}}</span>{{.Title}}</a>{{else}}<span><span class="n">{{.Number}}</span>{{.Title}}<span class="tag">{{if eq .State "locked"}}paid{{else if eq .State "failed"}}failed{{else if eq .State "gone"}}not on the site{{else}}not saved{{end}}</span></span>{{end}}
</li>{{end}}</ol>
<nav class="pager" id="pager" aria-label="Chapter pages" hidden></nav>
</main>
<script>
(function(){
 try{var s=JSON.parse(localStorage.getItem('na-reader')||'{}'),app=localStorage.getItem('na-theme')||''; if(s.theme)document.documentElement.dataset.theme=s.theme; else if(app&&app!=='system')document.documentElement.dataset.theme=app;}catch(e){}
 try{var last=localStorage.getItem('na-last-{{.Book.Slug}}'); if(last){var c=document.getElementById('continue'); c.href=last; c.textContent='Continue · chapter '+last;}}catch(e){}
 // 100 chapters a page; opens on the page with the chapter you're on.
 var q=document.getElementById('q'), items=[].slice.call(document.querySelectorAll('#list li'));
 var PAGE=100, page=0, pagers=[document.getElementById('pagerTop'),document.getElementById('pager')];
 try{var at=parseInt(localStorage.getItem('na-last-{{.Book.Slug}}')||'0',10); if(at>0){for(var i=0;i<items.length;i++){if(parseInt(items[i].dataset.t,10)>=at){page=Math.floor(i/PAGE);break}}}}catch(e){}
 function show(){
   var v=q.value.toLowerCase().trim();
   var m=items.filter(function(li){return !v||li.dataset.t.toLowerCase().indexOf(v)>=0});
   var pages=Math.max(1,Math.ceil(m.length/PAGE)); if(page>pages-1)page=pages-1;
   items.forEach(function(li){li.style.display='none'});
   m.slice(page*PAGE,page*PAGE+PAGE).forEach(function(li){li.style.display=''});
   var html='';
   if(pages>1){
     var o='';for(var k=0;k<pages;k++){o+='<option value="'+k+'"'+(k===page?' selected':'')+'>'+parseInt(m[k*PAGE].dataset.t,10)+'–'+parseInt(m[Math.min(m.length,(k+1)*PAGE)-1].dataset.t,10)+'</option>'}
     html='<button type="button" data-p="'+(page-1)+'"'+(page===0?' disabled':'')+' aria-label="Previous 100 chapters">← Prev</button>'
       +'<span><select aria-label="Show chapters">'+o+'</select>of '+m.length+'</span>'
       +'<button type="button" data-p="'+(page+1)+'"'+(page>=pages-1?' disabled':'')+' aria-label="Next 100 chapters">Next →</button>';
   }
   pagers.forEach(function(p){p.hidden=!html;p.innerHTML=html});
 }
 function go(n){page=n;show();var t=document.getElementById('pagerTop');if(t.getBoundingClientRect().top<0)t.scrollIntoView()}
 pagers.forEach(function(p){
   p.addEventListener('click',function(e){var b=e.target.closest('button');if(b&&!b.disabled)go(parseInt(b.dataset.p,10))});
   p.addEventListener('change',function(e){go(parseInt(e.target.value,10))});
 });
 q.addEventListener('input',function(){page=0;show()});
 show();
})();
</script></body></html>`))
