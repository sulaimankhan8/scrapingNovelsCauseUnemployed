package service

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"novelarchiver/internal/library"
)

// A backup is a zip:
//
//	manifest.json             BackupInfo
//	settings.json             Settings
//	library/<slug>/...        every file of every novel (book.json, chapters,
//	                          images, cover, reading.json)

// BackupInfo describes a backup file.
type BackupInfo struct {
	App       string    `json:"app"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	Novels    int       `json:"novels"`
	Chapters  int       `json:"chapters"`
	Bytes     int64     `json:"bytes"`
	// Filled in by InspectBackup for this device:
	Path     string `json:"path,omitempty"`
	FileName string `json:"fileName,omitempty"`
	NewHere  int    `json:"newHere"`
	Merged   int    `json:"merged"`
	SizeMb   string `json:"sizeMb,omitempty"`
}

// BackupFileName suggests a name for today's backup.
func BackupFileName() string {
	return "InkShelfs backup " + time.Now().Format("2006-01-02") + ".zip"
}

// Backup writes the whole library and settings to dest. Progress arrives as
// "backup" events {done, total} in bytes.
func (a *Service) Backup(dest string) (*BackupInfo, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	books, err := a.store.List()
	if err != nil {
		return nil, err
	}
	info := &BackupInfo{App: "NovelArchiver", Version: 1, CreatedAt: time.Now(), Novels: len(books)}
	for _, b := range books {
		ok, _, _, _ := b.Counts()
		info.Chapters += ok
		info.Bytes += a.store.Size(b.Slug)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	zw := zip.NewWriter(f)
	fail := func(err error) (*BackupInfo, error) {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return nil, err
	}
	writeJSON := func(name string, v any) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		return json.NewEncoder(w).Encode(v)
	}
	if err := writeJSON("manifest.json", info); err != nil {
		return fail(err)
	}
	if err := writeJSON("settings.json", a.GetSettings()); err != nil {
		return fail(err)
	}
	var done int64
	last := time.Time{}
	for _, b := range books {
		root := filepath.Join(a.store.Root, b.Slug)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasSuffix(p, ".tmp") {
				return err
			}
			rel, err := filepath.Rel(a.store.Root, p)
			if err != nil {
				return err
			}
			// Images and chapters are already compressed or tiny; Store
			// keeps the backup fast.
			method := zip.Deflate
			if ext := strings.ToLower(filepath.Ext(p)); ext == ".jpg" || ext == ".png" || ext == ".webp" || ext == ".gif" {
				method = zip.Store
			}
			w, err := zw.CreateHeader(&zip.FileHeader{Name: "library/" + filepath.ToSlash(rel), Method: method, Modified: time.Now()})
			if err != nil {
				return err
			}
			src, err := os.Open(p)
			if err != nil {
				return err
			}
			n, err := io.Copy(w, src)
			src.Close()
			done += n
			if time.Since(last) > 250*time.Millisecond {
				last = time.Now()
				a.emitRaw("backup", map[string]any{"done": done, "total": info.Bytes})
			}
			return err
		})
		if err != nil {
			return fail(err)
		}
	}
	if err := zw.Close(); err != nil {
		return fail(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	a.emitRaw("backup", map[string]any{"done": info.Bytes, "total": info.Bytes})
	info.Path, info.FileName = dest, filepath.Base(dest)
	if st, err := os.Stat(dest); err == nil {
		info.SizeMb = mb(st.Size())
	}
	return info, nil
}

func mb(n int64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

// backupBooks reads every book.json inside an open backup.
func backupBooks(zr *zip.Reader) (map[string]*library.Book, *BackupInfo, error) {
	books := map[string]*library.Book{}
	var info *BackupInfo
	for _, f := range zr.File {
		switch {
		case f.Name == "manifest.json":
			var m BackupInfo
			if err := readZipJSON(f, &m); err == nil {
				info = &m
			}
		case strings.HasPrefix(f.Name, "library/") && strings.HasSuffix(f.Name, "/book.json"):
			parts := strings.Split(f.Name, "/")
			if len(parts) != 3 || parts[1] != library.Slugify(parts[1]) {
				continue
			}
			var b library.Book
			if err := readZipJSON(f, &b); err == nil && b.Slug == parts[1] {
				books[parts[1]] = &b
			}
		}
	}
	if info == nil || info.App != "NovelArchiver" {
		return nil, nil, errors.New("this file isn't an InkShelfs (or NovelArchiver) backup")
	}
	return books, info, nil
}

func readZipJSON(f *zip.File, v any) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return json.NewDecoder(io.LimitReader(rc, 64<<20)).Decode(v)
}

// InspectBackup reads a backup's manifest and says what restoring would do.
func (a *Service) InspectBackup(src string) (*BackupInfo, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, errors.New("couldn't open that file as a backup")
	}
	defer zr.Close()
	books, info, err := backupBooks(&zr.Reader)
	if err != nil {
		return nil, err
	}
	info.Novels = len(books)
	for _, b := range books {
		if a.localSlug(b) != "" {
			info.Merged++
		} else {
			info.NewHere++
		}
	}
	info.Path, info.FileName, info.SizeMb = src, filepath.Base(src), mb(info.Bytes)
	return info, nil
}

// localSlug finds the stored copy of a backed-up book (same source link, or
// same folder name).
func (a *Service) localSlug(b *library.Book) string {
	if b.URL != "" {
		if s := a.store.FindByURL(b.URL); s != "" {
			return s
		}
	}
	if _, err := a.store.Load(b.Slug); err == nil {
		return b.Slug
	}
	return ""
}

// RestoreResult reports a restore.
type RestoreResult struct {
	Added    int `json:"added"`
	Merged   int `json:"merged"`
	Chapters int `json:"chapters"` // chapters added to novels you already had
}

// Restore merges a backup into the library. Nothing local is deleted:
// novels you don't have are added whole; for novels you have, chapters you
// are missing are added and the further reading place is kept.
func (a *Service) Restore(src string) (*RestoreResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	busy := a.cur != nil
	a.mu.Unlock()
	if busy {
		return nil, errors.New("wait for the current download to finish, or stop it, before restoring")
	}
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, errors.New("couldn't open that file as a backup")
	}
	defer zr.Close()
	books, _, err := backupBooks(&zr.Reader)
	if err != nil {
		return nil, err
	}
	files := map[string][]*zip.File{} // backup slug -> its files
	var settingsFile *zip.File
	for _, f := range zr.File {
		if f.Name == "settings.json" {
			settingsFile = f
			continue
		}
		parts := strings.SplitN(f.Name, "/", 3)
		if len(parts) == 3 && parts[0] == "library" && books[parts[1]] != nil {
			files[parts[1]] = append(files[parts[1]], f)
		}
	}
	res := &RestoreResult{}
	var total, done int64
	for _, list := range files {
		for _, f := range list {
			total += int64(f.UncompressedSize64)
		}
	}
	progress := func(n int64) {
		done += n
		a.emitRaw("backup", map[string]any{"done": done, "total": total, "restore": true})
	}

	for slug, b := range books {
		local := a.localSlug(b)
		if local == "" {
			dst := slug
			if _, err := os.Stat(filepath.Join(a.store.Root, dst)); err == nil {
				dst = a.store.NewSlug(b.Title)
			}
			for _, f := range files[slug] {
				rel := strings.TrimPrefix(f.Name, "library/"+slug+"/")
				n, err := a.extract(f, dst, rel, false)
				if err != nil {
					return res, err
				}
				progress(n)
			}
			if dst != slug {
				if nb, err := a.store.Load(dst); err == nil {
					nb.Slug = dst
					_ = a.store.Save(nb)
				}
			}
			res.Added++
			continue
		}
		// Merge into the copy on this device.
		lb, err := a.store.Load(local)
		if err != nil {
			continue
		}
		have := map[int]int{} // number -> index in lb.Chapters
		for i, c := range lb.Chapters {
			have[c.Number] = i
		}
		want := map[string]bool{}
		for _, c := range b.Chapters {
			i, ok := have[c.Number]
			if c.State == library.StateOK && ok && lb.Chapters[i].State != library.StateOK {
				want[fmt.Sprintf("chapters/%04d.html", c.Number)] = true
				lb.Chapters[i].State, lb.Chapters[i].Words, lb.Chapters[i].Error = library.StateOK, c.Words, ""
				res.Chapters++
			}
		}
		for _, f := range files[slug] {
			rel := strings.TrimPrefix(f.Name, "library/"+slug+"/")
			switch {
			case want[rel]:
				n, err := a.extract(f, local, rel, true)
				if err != nil {
					return res, err
				}
				progress(n)
			case strings.HasPrefix(rel, "img/"):
				n, _ := a.extract(f, local, rel, false) // only if missing
				progress(n)
			case rel == "reading.json":
				var r library.Reading
				if readZipJSON(f, &r) == nil && r.Chapter > a.store.Reading(local).Chapter {
					_ = a.store.SetReading(local, r.Chapter)
				}
			case strings.HasPrefix(rel, "cover") && lb.CoverFile == "":
				if n, err := a.extract(f, local, rel, false); err == nil {
					lb.CoverFile = rel
					progress(n)
				}
			}
		}
		_ = a.store.Save(lb)
		res.Merged++
	}
	// Settings only come back on a device that never saved its own.
	if settingsFile != nil {
		if _, err := os.Stat(a.settingsPath()); errors.Is(err, fs.ErrNotExist) {
			var s Settings
			if readZipJSON(settingsFile, &s) == nil {
				_, _ = a.SaveSettings(s)
			}
		}
	}
	return res, nil
}

// extract writes one backup file into a book folder. Unless overwrite is
// set, an existing file is left alone.
func (a *Service) extract(f *zip.File, slug, rel string, overwrite bool) (int64, error) {
	rel = path.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") {
		return 0, nil
	}
	dst, err := a.store.FilePath(slug, rel)
	if err != nil {
		return 0, nil
	}
	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return 0, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	out, err := os.Create(dst + ".tmp")
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(rc, 512<<20))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst + ".tmp")
		return n, err
	}
	return n, os.Rename(dst+".tmp", dst)
}
