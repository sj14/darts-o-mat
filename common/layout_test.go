package common_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/sj14/darts-o-mat/"

// The project is one Go module, so the compiler doesn't keep the programs apart: this does.
// The server and the camera share only what is in common; common knows neither of them.
func TestImports(t *testing.T) {
	forbidden := map[string][]string{
		"common": {"server", "cam"},
		"server": {"cam"},
		"cam":    {"server"},
	}
	for dir, others := range forbidden {
		err := filepath.WalkDir(filepath.Join("..", dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				for _, other := range others {
					if p == module+other || strings.HasPrefix(p, module+other+"/") {
						t.Errorf("%s imports %s: %s must not depend on %s", path, p, dir, other)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
