package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var imageReferencePattern = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)|<img[^>]+src=["']([^"']+)["'][^>]*>`)
var imageSuffixes = []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg", ".ico"}

func walkPages(root Page) []Page {
	var pages []Page
	var walk func(Page)
	walk = func(current Page) {
		pages = append(pages, current)
		children := append([]Page(nil), current.SubPages...)
		sort.SliceStable(children, func(i, j int) bool {
			if children[i].Order == children[j].Order {
				return children[i].Path < children[j].Path
			}
			return children[i].Order < children[j].Order
		})
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
	return pages
}

func atomicWrite(filePath string, content []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filePath), ".ado-wiki-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(perm); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, filePath)
}

func clearOutputDirectory(outputDirectory string) error {
	absolutePath, err := filepath.Abs(outputDirectory)
	if err != nil {
		return err
	}
	if err := rejectProtectedOutputDirectory(absolutePath); err != nil {
		return err
	}
	if err := os.MkdirAll(absolutePath, 0o755); err != nil {
		return err
	}
	info, err := os.Lstat(absolutePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output directory must not be a symbolic link: %s", absolutePath)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path is not a directory: %s", absolutePath)
	}
	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return err
	}
	if err := rejectProtectedOutputDirectory(resolvedPath); err != nil {
		return err
	}
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(resolvedPath, entry.Name())); err != nil {
			return fmt.Errorf("clear output directory entry %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func rejectProtectedOutputDirectory(outputDirectory string) error {
	protectedPaths := []string{string(filepath.Separator)}
	if workingDirectory, err := os.Getwd(); err == nil {
		protectedPaths = append(protectedPaths, workingDirectory)
	} else {
		return err
	}
	if homeDirectory, err := os.UserHomeDir(); err == nil {
		protectedPaths = append(protectedPaths, homeDirectory)
	}
	for _, protectedPath := range protectedPaths {
		protectedPath, err := filepath.Abs(protectedPath)
		if err != nil {
			return err
		}
		if resolved, err := filepath.EvalSymlinks(protectedPath); err == nil {
			protectedPath = resolved
		}
		relativePath, err := filepath.Rel(outputDirectory, protectedPath)
		if err != nil {
			return err
		}
		if relativePath == "." || (relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))) {
			return fmt.Errorf("refusing to clear output directory %q because it contains a protected location", outputDirectory)
		}
	}
	return nil
}

func looksLikeImage(reference string) bool {
	lowered := strings.ToLower(reference)
	if strings.HasPrefix(lowered, "data:image/") || strings.Contains(lowered, "/attachments/") || strings.Contains(lowered, "/images/") {
		return true
	}
	for _, suffix := range imageSuffixes {
		if strings.HasSuffix(lowered, suffix) {
			return true
		}
	}
	return false
}

func rewriteMarkdownImages(client *AzureDevOpsClient, pagePath, markdown, outputDirectory, rootFilename string) (string, []string) {
	localPage, err := PagePathToLocal(pagePath, rootFilename)
	if err != nil {
		return markdown, nil
	}
	pageDir := filepath.Dir(filepath.Join(outputDirectory, filepath.FromSlash(localPage)))
	mediaDir := filepath.Join(pageDir, strings.TrimSuffix(filepath.Base(localPage), path.Ext(localPage))+".media")
	var imagePaths []string
	rewritten := imageReferencePattern.ReplaceAllStringFunc(markdown, func(match string) string {
		groups := imageReferencePattern.FindStringSubmatch(match)
		reference := groups[1]
		if reference == "" {
			reference = groups[2]
		}
		if reference == "" || strings.HasPrefix(reference, "data:") || strings.HasPrefix(reference, "mailto:") || strings.HasPrefix(reference, "#") {
			return match
		}
		parsed, parseErr := url.Parse(reference)
		if parseErr != nil || (parsed.Scheme == "" && !strings.HasPrefix(reference, "/") && !strings.HasPrefix(reference, "./")) || !looksLikeImage(reference) {
			return match
		}
		filename, decodeErr := url.PathUnescape(path.Base(parsed.Path))
		if decodeErr != nil || filename == "." || filename == "/" || filename == "" {
			return match
		}
		filename = strings.NewReplacer("/", "_", "\\", "_").Replace(filename)
		assetPath := filepath.Join(mediaDir, filename)
		content, downloadErr := client.DownloadBinary(reference)
		if downloadErr != nil || atomicWrite(assetPath, content, 0o644) != nil {
			return match
		}
		relative, relErr := filepath.Rel(pageDir, assetPath)
		if relErr != nil {
			return match
		}
		relative = filepath.ToSlash(relative)
		if !strings.HasPrefix(relative, ".") {
			relative = "./" + relative
		}
		imagePaths = append(imagePaths, relative)
		return strings.Replace(match, reference, relative, 1)
	})
	return rewritten, imagePaths
}

func ExportWiki(client *AzureDevOpsClient, project string, wiki Wiki, outputDirectory, rootFilename string, onPage func(Page, string)) (ExportManifest, error) {
	return ExportWikiFromPage(client, project, wiki, "/", outputDirectory, rootFilename, onPage)
}

func ExportWikiFromPage(client *AzureDevOpsClient, project string, wiki Wiki, startPage, outputDirectory, rootFilename string, onPage func(Page, string)) (ExportManifest, error) {
	manifest := newManifest(client.Organization(), project, wiki, outputDirectory)
	if _, err := PagePathToLocal(startPage, rootFilename); err != nil {
		return manifest, err
	}
	startPage = path.Clean(startPage)
	root, err := client.GetPage(project, wiki, startPage)
	if err != nil {
		return manifest, err
	}
	if err := clearOutputDirectory(outputDirectory); err != nil {
		return manifest, err
	}
	writtenPaths := make(map[string]struct{})
	for _, page := range walkPages(root) {
		if startPage != "/" && page.Path != startPage && !strings.HasPrefix(page.Path, startPage+"/") {
			continue
		}
		content, etag, pageErr := client.GetRawPage(project, wiki, page.Path)
		if pageErr == nil {
			localPath, pathErr := PagePathToLocal(page.Path, rootFilename)
			pageErr = pathErr
			if pageErr == nil {
				if _, exists := writtenPaths[localPath]; exists {
					pageErr = fmt.Errorf("multiple wiki pages map to %s", localPath)
				} else {
					content, images := rewriteMarkdownImages(client, page.Path, content, outputDirectory, rootFilename)
					pageErr = atomicWrite(filepath.Join(outputDirectory, filepath.FromSlash(localPath)), []byte(content), 0o644)
					if pageErr == nil {
						writtenPaths[localPath] = struct{}{}
						manifest.Pages = append(manifest.Pages, ManifestPage{SourcePath: page.Path, LocalPath: localPath, Order: page.Order, ETag: etag, Images: images})
						if onPage != nil {
							onPage(page, localPath)
						}
					}
				}
			}
		}
		if pageErr != nil {
			manifest.Failures = append(manifest.Failures, ManifestFailure{SourcePath: page.Path, Error: pageErr.Error()})
		}
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	encoded = append(encoded, '\n')
	if err := atomicWrite(filepath.Join(outputDirectory, ".wiki-cli.json"), encoded, 0o644); err != nil {
		return manifest, err
	}
	return manifest, nil
}
