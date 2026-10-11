package main

type Wiki struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsDisabled bool   `json:"isDisabled"`
}

type Page struct {
	Path     string  `json:"path"`
	Order    int     `json:"order"`
	SubPages []Page  `json:"subPages"`
	ETag     *string `json:"-"`
}

type ManifestWiki struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type ManifestPage struct {
	SourcePath string   `json:"sourcePath"`
	LocalPath  string   `json:"localPath"`
	Order      int      `json:"order"`
	ETag       *string  `json:"etag"`
	Images     []string `json:"images"`
}

type ManifestFailure struct {
	SourcePath string `json:"sourcePath"`
	Error      string `json:"error"`
}

type ExportManifest struct {
	Organization    string            `json:"organization"`
	Project         string            `json:"project"`
	Wiki            ManifestWiki      `json:"wiki"`
	OutputDirectory string            `json:"outputDirectory"`
	Pages           []ManifestPage    `json:"pages"`
	Failures        []ManifestFailure `json:"failures"`
}

func newManifest(organization, project string, wiki Wiki, outputDirectory string) ExportManifest {
	return ExportManifest{
		Organization:    organization,
		Project:         project,
		Wiki:            ManifestWiki{ID: wiki.ID, Name: wiki.Name, Type: wiki.Type},
		OutputDirectory: outputDirectory,
		Pages:           []ManifestPage{},
		Failures:        []ManifestFailure{},
	}
}
