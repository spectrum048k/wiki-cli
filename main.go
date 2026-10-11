package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
)

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("wiki-cli", flag.ContinueOnError)
	flags.SetOutput(stderr)
	organization := flags.String("organization", "", "Azure DevOps organization")
	project := flags.String("project", "", "Azure DevOps project")
	wikiName := flags.String("wiki", "", "Wiki name or ID. Required when the project has multiple enabled wikis.")
	baseURL := flags.String("base-url", "", "Azure DevOps API base URL before the project path (default https://dev.azure.com/<organization>)")
	startPage := flags.String("start-page", "/", "Wiki page path to export, including its subpages")
	output := flags.String("output", "./exported-wiki", "Output directory")
	rootFilename := flags.String("root-filename", "_index.md", "Filename for the root wiki page")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *organization == "" || *project == "" {
		fmt.Fprintln(stderr, "--organization and --project are required")
		return 2
	}
	apiBaseURL := "https://dev.azure.com/" + url.PathEscape(*organization)
	if *baseURL != "" {
		var err error
		apiBaseURL, err = normalizeAzureDevOpsBaseURL(*baseURL)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	token := os.Getenv("AZDO_TOKEN")
	if token == "" {
		fmt.Fprintln(stderr, "AZDO_TOKEN is required")
		return 2
	}

	client := NewAzureDevOpsClientWithBaseURL(*organization, token, apiBaseURL)
	wikis, err := client.ListWikis(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	wiki, err := selectWiki(wikis, *wikiName, *project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	manifest, err := ExportWikiFromPage(client, *project, wiki, *startPage, *output, *rootFilename, func(_ Page, localPath string) {
		fmt.Fprintln(stdout, localPath)
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Exported %d pages to %s\n", len(manifest.Pages), *output)
	if len(manifest.Failures) > 0 {
		fmt.Fprintf(stderr, "%d pages failed\n", len(manifest.Failures))
		return 1
	}
	return 0
}

func selectWiki(wikis []Wiki, wikiName, project string) (Wiki, error) {
	var matches []Wiki
	for _, wiki := range wikis {
		if wikiName == "" || wikiName == wiki.ID || wikiName == wiki.Name {
			matches = append(matches, wiki)
		}
	}
	if wikiName != "" {
		if len(matches) == 0 {
			return Wiki{}, fmt.Errorf("no wiki found matching --wiki %q", wikiName)
		}
		if len(matches) != 1 {
			return Wiki{}, fmt.Errorf("--wiki %q matches multiple wikis; specify a unique wiki ID", wikiName)
		}
		if matches[0].IsDisabled {
			return Wiki{}, fmt.Errorf("wiki is disabled: %s", matches[0].Name)
		}
		return matches[0], nil
	}

	var enabled []Wiki
	for _, wiki := range matches {
		if !wiki.IsDisabled {
			enabled = append(enabled, wiki)
		}
	}
	switch len(enabled) {
	case 0:
		return Wiki{}, fmt.Errorf("no enabled wikis found in project %q", project)
	case 1:
		return enabled[0], nil
	default:
		return Wiki{}, fmt.Errorf("specify --wiki; the project has multiple enabled wikis")
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
