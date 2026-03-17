// internal/api/opml.go
package api

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Body    opmlBody `xml:"body"`
}
type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}
type opmlOutline struct {
	Text   string `xml:"text,attr"`
	Type   string `xml:"type,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

func (s *Server) opmlImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, 400, "multipart parse error")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "file field required")
		return
	}
	defer file.Close()
	data, _ := io.ReadAll(file)
	var doc opmlDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		writeError(w, 400, "invalid OPML")
		return
	}
	var imported, skipped int
	for _, o := range doc.Body.Outlines {
		if o.XMLURL == "" {
			skipped++
			continue
		}
		if _, err := s.store.CreateFeed(r.Context(), o.XMLURL); err != nil {
			skipped++
			continue
		}
		imported++
	}
	writeJSON(w, 200, map[string]int{"imported": imported, "skipped": skipped})
}

func (s *Server) opmlExport(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.store.ListFeeds(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	doc := opmlDoc{Version: "2.0"}
	for _, f := range feeds {
		title := f.URL
		if f.Title != nil {
			title = *f.Title
		}
		doc.Body.Outlines = append(doc.Body.Outlines, opmlOutline{Text: title, Type: "rss", XMLURL: f.URL})
	}
	out, _ := xml.MarshalIndent(doc, "", "  ")
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", `attachment; filename="podcatcher.opml"`)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>%s`, string(out))
}
