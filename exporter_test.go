package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExportWikiPreservesMarkdownAndDownloadsImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/project/_apis/wiki/wikis/wiki-id/pages" && request.Header.Get("Accept") == "text/plain" {
			response.Header().Set("ETag", "etag-1")
			if request.URL.Query().Get("path") == "/Guides/Setup" {
				response.Write([]byte("![Diagram](" + "http://" + request.Host + "/images/diagram.png)\n# raw\n"))
			} else {
				response.Write([]byte("# root\n"))
			}
			return
		}
		if request.URL.Path == "/images/diagram.png" {
			response.Write([]byte("PNGDATA"))
			return
		}
		if request.URL.Path == "/project/_apis/wiki/wikis/wiki-id/pages" {
			response.Write([]byte(`{"path":"/","subPages":[{"path":"/Guides/Setup","order":1,"subPages":[]}]}`))
			return
		}
		http.NotFound(response, request)
	}))
	defer server.Close()

	client := newAzureDevOpsClient("org", "token", server.URL)
	client.httpClient = server.Client()
	output := t.TempDir()
	staleFile := filepath.Join(output, "old-export", "stale.md")
	if err := os.MkdirAll(filepath.Dir(staleFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staleFile, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := ExportWiki(client, "project", Wiki{ID: "wiki-id", Name: "Docs", Type: "projectWiki"}, output, "_index.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("stale output file was not removed: %v", err)
	}
	markdown, err := os.ReadFile(filepath.Join(output, "Guides", "Setup.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(markdown) != "![Diagram](./Setup.media/diagram.png)\n# raw\n" {
		t.Fatalf("unexpected markdown: %q", markdown)
	}
	image, err := os.ReadFile(filepath.Join(output, "Guides", "Setup.media", "diagram.png"))
	if err != nil || string(image) != "PNGDATA" {
		t.Fatalf("unexpected image content %q, err %v", image, err)
	}
	if len(manifest.Pages) != 2 || manifest.Pages[0].SourcePath != "/" || manifest.Pages[1].SourcePath != "/Guides/Setup" ||
		manifest.Pages[0].ETag == nil || *manifest.Pages[0].ETag != "etag-1" ||
		manifest.Pages[1].ETag == nil || *manifest.Pages[1].ETag != "etag-1" || len(manifest.Failures) != 0 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	rootMarkdown, err := os.ReadFile(filepath.Join(output, "_index.md"))
	if err != nil || string(rootMarkdown) != "# root\n" {
		t.Fatalf("unexpected root markdown %q, err %v", rootMarkdown, err)
	}
	if _, err := os.Stat(filepath.Join(output, ".wiki-cli.json")); err != nil {
		t.Fatal(err)
	}
}

func TestClearOutputDirectoryRejectsWorkingDirectory(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := clearOutputDirectory(workingDirectory); err == nil {
		t.Fatal("clearOutputDirectory() should reject the working directory")
	}
}

func TestExportWikiFromPageIncludesOnlySelectedPageAndDescendants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/project/_apis/wiki/wikis/wiki-id/pages" {
			http.NotFound(response, request)
			return
		}
		if request.Header.Get("Accept") == "application/json" {
			if request.URL.Query().Get("path") != "/Guides/Start" || request.URL.Query().Get("recursionLevel") != "Full" {
				t.Errorf("unexpected page-tree request: %s", request.URL.RawQuery)
			}
			response.Write([]byte(`{"path":"/Guides/Start","subPages":[{"path":"/Guides/Start/Child","subPages":[{"path":"/Guides/Start/Child/Grandchild"}]},{"path":"/Elsewhere"}]}`))
			return
		}
		response.Write([]byte("content: " + request.URL.Query().Get("path")))
	}))
	defer server.Close()

	client := newAzureDevOpsClient("org", "token", server.URL)
	client.httpClient = server.Client()
	output := t.TempDir()
	var exportedPaths []string
	manifest, err := ExportWikiFromPage(client, "project", Wiki{ID: "wiki-id", Name: "Docs"}, "/Guides/Start", output, "_index.md", func(_ Page, localPath string) {
		exportedPaths = append(exportedPaths, localPath)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Pages) != 3 || len(manifest.Failures) != 0 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	wantPaths := []string{"Guides/Start.md", "Guides/Start/Child.md", "Guides/Start/Child/Grandchild.md"}
	if len(exportedPaths) != len(wantPaths) {
		t.Fatalf("export callback paths = %#v, want %#v", exportedPaths, wantPaths)
	}
	for index, relativePath := range wantPaths {
		if exportedPaths[index] != relativePath {
			t.Errorf("export callback path[%d] = %q, want %q", index, exportedPaths[index], relativePath)
		}
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(relativePath))); err != nil {
			t.Errorf("expected exported page %s: %v", relativePath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "Elsewhere.md")); !os.IsNotExist(err) {
		t.Errorf("unrelated page was exported or stat failed unexpectedly: %v", err)
	}
}
