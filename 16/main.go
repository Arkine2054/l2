package main

import (
	"crypto/sha1"
	"flag"
	"fmt"
	"github.com/temoto/robotstxt"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// go run main.go -url https://example.com -out ./mirror_example -depth 2 -parallel 16 -timeout 15s
type Mirror struct {
	root       *url.URL
	outDir     string
	depth      int
	client     *http.Client
	visited    map[string]struct{}
	visitedMu  sync.Mutex
	wg         sync.WaitGroup
	sem        chan struct{}
	domainHost string
	robots     *robotstxt.RobotsData
	agentName  string
}

func NewMirror(root *url.URL, outDir string, depth int, parallel int, timeout time.Duration) *Mirror {
	if parallel < 1 {
		parallel = 4
	}
	client := &http.Client{
		Timeout: timeout,
	}
	return &Mirror{
		root:       root,
		outDir:     outDir,
		depth:      depth,
		client:     client,
		visited:    make(map[string]struct{}),
		sem:        make(chan struct{}, parallel),
		domainHost: root.Host,
		agentName:  "GoMirror",
	}
}

func (m *Mirror) enqueueURL(u *url.URL, curDepth int) {
	n := m.normalize(u)
	m.visitedMu.Lock()
	if _, ok := m.visited[n]; ok {
		m.visitedMu.Unlock()
		return
	}
	m.visited[n] = struct{}{}
	m.visitedMu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.sem <- struct{}{}
		defer func() { <-m.sem }()
		if curDepth > m.depth {
			return
		}
		if err := m.fetchAndProcess(u, curDepth); err != nil {
			log.Printf("error processing %s: %v\n", u.String(), err)
		}
	}()
}

func (m *Mirror) normalize(u *url.URL) string {
	nu := *u
	nu.Fragment = ""
	if nu.Path == "" {
		nu.Path = "/"
	}
	return nu.String()
}

func (m *Mirror) fetchAndProcess(u *url.URL, curDepth int) error {
	if m.robots != nil {
		group := m.robots.FindGroup(m.agentName)
		if !group.Test(u.Path) {
			log.Printf("[robots.txt] disallowed: %s", u.String())
			return nil
		}
	}

	if u.Host != m.domainHost {
		return nil
	}

	resp, err := m.client.Get(u.String())
	if err != nil {
		return fmt.Errorf("GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") || maybeHTMLByURL(u.Path) {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return m.processHTML(u, body, curDepth)
	}

	return m.saveResource(u, resp.Body)
}

func (m *Mirror) loadRobotsTxt() {
	robotsURL := *m.root
	robotsURL.Path = "/robots.txt"
	resp, err := m.client.Get(robotsURL.String())
	if err != nil {
		log.Printf("[robots.txt] failed to fetch: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[robots.txt] not found (%d)", resp.StatusCode)
		return
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[robots.txt] read error: %v", err)
		return
	}

	robots, err := robotstxt.FromBytes(data)
	if err != nil {
		log.Printf("[robots.txt] parse error: %v", err)
		return
	}

	m.robots = robots
	log.Printf("[robots.txt] loaded for %s", m.root.Host)
}

func maybeHTMLByURL(path string) bool {
	lpath := strings.ToLower(path)
	if strings.HasSuffix(lpath, ".html") || strings.HasSuffix(lpath, ".htm") || strings.HasSuffix(lpath, "/") || lpath == "" {
		return true
	}
	return false
}

func (m *Mirror) saveResource(u *url.URL, r io.Reader) error {
	localPath := m.urlToFilePath(u)
	fullPath := filepath.Join(m.outDir, localPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(fullPath); err == nil {
		return nil
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	if err != nil {
		_ = os.Remove(fullPath)
		return err
	}
	log.Printf("saved resource: %s -> %s\n", u.String(), fullPath)
	return nil
}

func (m *Mirror) urlToFilePath(u *url.URL) string {
	cleanPath := u.Path
	if cleanPath == "" || strings.HasSuffix(cleanPath, "/") {
		cleanPath = filepath.Join(cleanPath, "index.html")
	}

	cleanPath = strings.TrimPrefix(cleanPath, "/")

	dir, file := filepath.Split(cleanPath)
	if file == "" {
		file = "index.html"
	}

	if !strings.Contains(file, ".") {
		cleanPath = filepath.Join(dir, file, "index.html")
	}

	if u.RawQuery != "" {
		h := sha1.Sum([]byte(u.RawQuery))
		hash := fmt.Sprintf("%x", h)[:10]
		ext := filepath.Ext(cleanPath)
		base := strings.TrimSuffix(cleanPath, ext)
		cleanPath = base + "_" + hash + ext
	}

	return filepath.Join(u.Host, cleanPath)
}

func (m *Mirror) processHTML(u *url.URL, body []byte, curDepth int) error {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	var mu sync.Mutex
	var toEnqueue []*url.URL

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := []string{}
			switch n.Data {
			case "a":
				attrs = []string{"href"}
			case "img", "script":
				attrs = []string{"src"}
			case "link":
				for _, a := range n.Attr {
					if a.Key == "rel" && (strings.Contains(a.Val, "stylesheet") || strings.Contains(a.Val, "icon")) {
						attrs = []string{"href"}
					}
				}
			}
			for i := range n.Attr {
				attr := &n.Attr[i]
				if contains(attrs, attr.Key) {
					raw := strings.TrimSpace(attr.Val)
					if raw == "" || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "javascript:") {
						continue
					}
					parsed, err := url.Parse(raw)
					if err != nil {
						continue
					}
					abs := m.root.ResolveReference(parsed)
					mu.Lock()
					toEnqueue = append(toEnqueue, abs)
					mu.Unlock()
					local := m.urlToFilePath(abs)
					curLocal := m.urlToFilePath(u)
					rel, err := filepath.Rel(filepath.Dir(filepath.Join(m.outDir, curLocal)), filepath.Join(m.outDir, local))
					if err != nil {
						attr.Val = "/" + filepath.ToSlash(local)
					} else {
						attr.Val = filepath.ToSlash(rel)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	localPath := m.urlToFilePath(u)
	fullPath := filepath.Join(m.outDir, localPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := html.Render(f, doc); err != nil {
		_ = os.Remove(fullPath)
		return err
	}
	log.Printf("saved page: %s -> %s\n", u.String(), fullPath)

	for _, found := range toEnqueue {
		if found.Host != m.domainHost {
			continue
		}

		if curDepth+1 <= m.depth {
			m.enqueueURL(found, curDepth+1)
		} else {
			m.enqueueURL(found, curDepth+1)
		}
	}

	return nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func main() {
	var (
		startURL string
		outDir   string
		depth    int
		parallel int
		timeout  time.Duration
		help     bool
	)
	flag.StringVar(&startURL, "url", "", "Start URL (required)")
	flag.StringVar(&outDir, "out", "mirror_out", "Output directory")
	flag.IntVar(&depth, "depth", 2, "Recursion depth (levels of links to follow)")
	flag.IntVar(&parallel, "parallel", 8, "Max parallel downloads")
	flag.DurationVar(&timeout, "timeout", 20*time.Second, "HTTP client timeout")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.Parse()

	if help || startURL == "" {
		flag.Usage()
		return
	}

	parsed, err := url.Parse(startURL)
	if err != nil {
		log.Fatalf("invalid url: %v", err)
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}

	m := NewMirror(parsed, outDir, depth, parallel, timeout)

	m.loadRobotsTxt()

	m.enqueueURL(parsed, 0)

	m.wg.Wait()
	log.Println("mirror finished")
}
