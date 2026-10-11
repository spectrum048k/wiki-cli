package main

import "testing"

func TestPagePathToLocal(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/Guides/Setup", want: "Guides/Setup.md"},
		{path: "/", want: "_index.md"},
		{path: "/API:Reference/CON", want: "API_Reference/_CON.md"},
		{path: "/A:B", want: "A_B.md"},
		{path: "/A?B", want: "A_B.md"},
		{path: "/.gitignore", want: ".gitignore.md"},
	}
	for _, test := range tests {
		got, err := PagePathToLocal(test.path, "_index.md")
		if err != nil || got != test.want {
			t.Errorf("PagePathToLocal(%q) = %q, %v; want %q", test.path, got, err, test.want)
		}
	}
}

func TestPagePathToLocalRejectsUnsafePaths(t *testing.T) {
	for _, path := range []string{"../secret", "/../secret", "/./secret"} {
		if _, err := PagePathToLocal(path, "_index.md"); err == nil {
			t.Errorf("PagePathToLocal(%q) should fail", path)
		}
	}
}

func TestWalkPagesUsesDepthFirstNavigationOrder(t *testing.T) {
	root := Page{Path: "/", SubPages: []Page{
		{Path: "/B", Order: 2, SubPages: []Page{{Path: "/B/Child", Order: 1}}},
		{Path: "/A", Order: 1},
	}}
	pages := walkPages(root)
	if len(pages) != 4 || pages[0].Path != "/" || pages[1].Path != "/A" || pages[2].Path != "/B" || pages[3].Path != "/B/Child" {
		t.Fatalf("unexpected page order: %#v", pages)
	}
}
