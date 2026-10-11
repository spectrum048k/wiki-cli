package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var invalidPathCharacters = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

var reservedPathNames = func() map[string]struct{} {
	names := map[string]struct{}{"CON": {}, "PRN": {}, "AUX": {}, "NUL": {}}
	for index := 1; index <= 9; index++ {
		names[fmt.Sprintf("COM%d", index)] = struct{}{}
		names[fmt.Sprintf("LPT%d", index)] = struct{}{}
	}
	return names
}()

func PagePathToLocal(pagePath, rootFilename string) (string, error) {
	if !strings.HasPrefix(pagePath, "/") {
		return "", fmt.Errorf("wiki page path must be absolute: %q", pagePath)
	}
	segments := strings.Split(pagePath, "/")
	var safe []string
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if segment == "." || segment == ".." || strings.ContainsRune(segment, 0) {
			return "", fmt.Errorf("unsafe wiki path segment: %q", segment)
		}
		cleaned := strings.TrimRight(strings.TrimSpace(invalidPathCharacters.ReplaceAllString(segment, "_")), ".")
		if cleaned == "" {
			return "", fmt.Errorf("empty wiki path segment: %q", segment)
		}
		stem := strings.ToUpper(strings.SplitN(cleaned, ".", 2)[0])
		if _, reserved := reservedPathNames[stem]; reserved {
			cleaned = "_" + cleaned
		}
		safe = append(safe, cleaned)
	}
	if len(safe) == 0 {
		return rootFilename, nil
	}
	localPath := path.Join(safe...)
	extension := path.Ext(localPath)
	base := path.Base(localPath)
	if strings.HasPrefix(base, ".") && strings.Count(base, ".") == 1 {
		extension = ""
	}
	return strings.TrimSuffix(localPath, extension) + ".md", nil
}
