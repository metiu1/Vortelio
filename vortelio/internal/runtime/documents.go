package runtime

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// RunCodeSnippet executes a code snippet locally and returns combined output.
func RunCodeSnippet(language, code string) (string, error) {
	lang := strings.ToLower(strings.TrimSpace(language))
	dir, err := os.MkdirTemp("", "vortelio-code-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	write := func(name string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(code), 0644)
		return p
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	look := func(names ...string) string {
		for _, n := range names {
			if p, err := exec.LookPath(n); err == nil {
				return p
			}
		}
		return ""
	}

	var cmd *exec.Cmd
	switch lang {
	case "python", "py", "python3":
		py := FindPython()
		if py == "" {
			return "", fmt.Errorf("Python non trovato")
		}
		cmd = exec.CommandContext(ctx, py, write("s.py"))
	case "javascript", "js", "node", "nodejs":
		if look("node", "node.exe") == "" {
			return "", fmt.Errorf("Node.js non trovato")
		}
		cmd = exec.CommandContext(ctx, "node", write("s.js"))
	case "bash", "sh", "shell":
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", code)
		} else {
			cmd = exec.CommandContext(ctx, "sh", write("s.sh"))
		}
	case "powershell", "ps", "ps1":
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", code)
	case "go", "golang":
		if look("go", "go.exe") == "" {
			return "", fmt.Errorf("Go non trovato")
		}
		cmd = exec.CommandContext(ctx, "go", "run", write("s.go"))
	case "ruby", "rb":
		if look("ruby", "ruby.exe") == "" {
			return "", fmt.Errorf("Ruby non trovato")
		}
		cmd = exec.CommandContext(ctx, "ruby", write("s.rb"))
	case "php":
		if look("php", "php.exe") == "" {
			return "", fmt.Errorf("PHP non trovato")
		}
		cmd = exec.CommandContext(ctx, "php", write("s.php"))
	case "java":
		if look("java", "java.exe") == "" {
			return "", fmt.Errorf("Java non trovato")
		}
		cmd = exec.CommandContext(ctx, "java", write("s.java"))
	case "c":
		gcc := look("gcc", "gcc.exe", "cc")
		if gcc == "" {
			return "", fmt.Errorf("gcc non trovato")
		}
		out := filepath.Join(dir, "a.exe")
		if b, e := exec.CommandContext(ctx, gcc, write("s.c"), "-o", out).CombinedOutput(); e != nil {
			return string(b), fmt.Errorf("compilazione fallita")
		}
		cmd = exec.CommandContext(ctx, out)
	case "cpp", "c++", "cc":
		gpp := look("g++", "g++.exe", "clang++")
		if gpp == "" {
			return "", fmt.Errorf("g++ non trovato")
		}
		out := filepath.Join(dir, "a.exe")
		if b, e := exec.CommandContext(ctx, gpp, write("s.cpp"), "-o", out).CombinedOutput(); e != nil {
			return string(b), fmt.Errorf("compilazione fallita")
		}
		cmd = exec.CommandContext(ctx, out)
	default:
		return "", fmt.Errorf("linguaggio non supportato: %s", lang)
	}
	cmd.Dir = dir
	cmd = HideWindow(cmd)
	out, runErr := cmd.CombinedOutput()
	s := string(out)
	if len(s) > 20000 {
		s = s[:20000] + "\n…[output troncato]"
	}
	if ctx.Err() == context.DeadlineExceeded {
		return s + "\n…[interrotto dopo 30s]", nil
	}
	return s, runErr
}

// CreateDocument writes content to a file in the given format. Supported:
// txt, md, html, csv, json (plain), docx, pdf. Returns the path written.
func CreateDocument(format, path, title, content string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "txt", "md", "markdown", "html", "htm", "csv", "json", "":
		body := content
		if format == "md" || format == "markdown" {
			if title != "" {
				body = "# " + title + "\n\n" + content
			}
		} else if format == "html" || format == "htm" {
			body = "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>" + htmlEsc(title) + "</title></head><body>\n" + content + "\n</body></html>"
		} else if title != "" {
			body = title + "\n\n" + content
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			return "", err
		}
		return path, nil
	case "docx":
		return path, writeDocx(path, title, content)
	case "pdf":
		return path, writePDF(path, title, content)
	default:
		return "", fmt.Errorf("formato non supportato: %s (usa txt, md, html, csv, json, docx, pdf)", format)
	}
}

func htmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// ── DOCX (minimal OpenXML, dependency-free) ─────────────────────────
func writeDocx(path, title, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	add := func(name, body string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(body))
		return err
	}

	if err := add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`); err != nil {
		return err
	}
	if err := add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`); err != nil {
		return err
	}

	var para strings.Builder
	xesc := func(s string) string {
		r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
		return r.Replace(s)
	}
	if title != "" {
		para.WriteString(`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="32"/></w:rPr><w:t xml:space="preserve">` + xesc(title) + `</w:t></w:r></w:p>`)
	}
	for _, line := range strings.Split(content, "\n") {
		para.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + xesc(line) + `</w:t></w:r></w:p>`)
	}
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>` + para.String() + `</w:body></w:document>`
	return add("word/document.xml", doc)
}

// ── PDF (minimal text, dependency-free) ─────────────────────────────
func writePDF(path, title, content string) error {
	// Lay out text into wrapped lines and paginate.
	const (
		pageW, pageH = 595, 842
		left, top    = 50.0, 800.0
		leading      = 14.0
		fontSize     = 11
		maxChars     = 95
		linesPerPage = 52
	)
	var allLines []string
	if title != "" {
		allLines = append(allLines, "## "+title) // marker for bold-ish (rendered larger)
		allLines = append(allLines, "")
	}
	for _, raw := range strings.Split(content, "\n") {
		allLines = append(allLines, wrapLine(raw, maxChars)...)
	}

	// Build content streams per page.
	var pages []string
	for i := 0; i < len(allLines); i += linesPerPage {
		end := i + linesPerPage
		if end > len(allLines) {
			end = len(allLines)
		}
		var b strings.Builder
		b.WriteString("BT\n")
		b.WriteString(fmt.Sprintf("/F1 %d Tf\n", fontSize))
		b.WriteString(fmt.Sprintf("%.0f %.0f Td\n", left, top))
		b.WriteString(fmt.Sprintf("%.0f TL\n", leading))
		for _, ln := range allLines[i:end] {
			size := fontSize
			text := ln
			if strings.HasPrefix(ln, "## ") {
				size = 16
				text = strings.TrimPrefix(ln, "## ")
			}
			if size != fontSize {
				b.WriteString(fmt.Sprintf("/F1 %d Tf\n", size))
			}
			b.WriteString("(" + pdfEsc(text) + ") Tj\n")
			b.WriteString("T*\n")
			if size != fontSize {
				b.WriteString(fmt.Sprintf("/F1 %d Tf\n", fontSize))
			}
		}
		b.WriteString("ET")
		pages = append(pages, b.String())
	}
	if len(pages) == 0 {
		pages = []string{"BT /F1 11 Tf 50 800 Td (." + ") Tj ET"}
	}

	// Assemble objects.
	// 1 Catalog, 2 Pages, 3 Font, then per page: page obj + content obj.
	var objs []string
	kids := []string{}
	pageObjStart := 4
	for i := range pages {
		pageNum := pageObjStart + i*2
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNum))
	}
	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>")
	objs = append(objs, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	objs = append(objs, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	for i, cs := range pages {
		pageNum := pageObjStart + i*2
		contentNum := pageNum + 1
		page := fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", pageW, pageH, contentNum)
		_ = page
		// page object
		objsAppendAt(&objs, pageNum, page)
		stream := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cs)+0, cs)
		objsAppendAt(&objs, contentNum, stream)
	}

	// Serialize with xref.
	var out strings.Builder
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs)+1)
	for i, body := range objs {
		offsets[i+1] = out.Len()
		out.WriteString(strconv.Itoa(i+1) + " 0 obj\n" + body + "\nendobj\n")
	}
	xrefPos := out.Len()
	out.WriteString("xref\n")
	out.WriteString(fmt.Sprintf("0 %d\n", len(objs)+1))
	out.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objs); i++ {
		out.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	out.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xrefPos))

	return os.WriteFile(path, []byte(out.String()), 0644)
}

// objsAppendAt ensures objs has at least n entries and sets index n-1.
func objsAppendAt(objs *[]string, n int, body string) {
	for len(*objs) < n {
		*objs = append(*objs, "<< >>")
	}
	(*objs)[n-1] = body
}

func wrapLine(s string, max int) []string {
	if s == "" {
		return []string{""}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= max {
			cur += " " + w
		} else {
			out = append(out, cur)
			cur = w
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// pdfEsc escapes a string for a PDF literal and maps to Latin-1 (Helvetica WinAnsi).
func pdfEsc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if r < 256 {
				b.WriteByte(byte(r))
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}
