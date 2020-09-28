package docs

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
)

type OpenAPIRequired int

const (
	Unknown OpenAPIRequired = iota
	Optional
	Required
)

func GetDocsForField(t reflect.Type, fieldName string) (string, OpenAPIRequired, error) {
	pkg := t.PkgPath()
	pm, err := makeParserMapForPackage(pkg)
	if err != nil {
		return "", Unknown, fmt.Errorf("unable to parse dir: %w", err)
	}
	p, present := pm[pkg]
	if !present {
		return "", Unknown, fmt.Errorf("package not present: %q", pkg)
	}
	dp := doc.New(p, pkg, 0)
	for _, dt := range dp.Types {
		if dt.Name == t.Name() {
			for _, spec := range dt.Decl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						if fieldName == name.Name {
							fieldDoc, isRequired := parseFieldDocs(field)
							return fieldDoc, isRequired, nil
						}
					}
				}
			}
		}
	}
	return "", Unknown, fmt.Errorf("did not find doc for %q", t.Name())
}

func ignoreDirectories(fi os.FileInfo) bool {
	return !fi.IsDir()
}

var parserMapCache = map[string]*ast.Package{}

func makeParserMapForPackage(pkg string) (map[string]*ast.Package, error) {
	fs := token.NewFileSet()
	// pList is the list of packages to parse. As we parse one package, we will often encounter
	// other packages that need to be parsed, adding them to this list.
	pList := []string{strings.Replace(pkg, "github.com/google/knative-gcp", ".", 1)}
	for len(pList) > 0 {
		current := pList[0]
		pList = pList[1:]
		if _, ok := parserMapCache[current]; ok {
			continue
		}
		if !strings.HasPrefix(current, "github.com/google/knative-gcp") &&
			!strings.HasPrefix(current, ".") &&
			!strings.HasPrefix(current, "vendor/") {
			current = "vendor/" + current
		}
		spm, err := parser.ParseDir(fs, current, ignoreDirectories, parser.ParseComments)
		if err != nil {
			return parserMapCache, fmt.Errorf("error parse dir %q: %w", current, err)
		}
		for _, v := range spm {
			localName := current
			if strings.HasPrefix(localName, "./") {
				localName = localName[2:]
			}
			name := fmt.Sprintf("%s/%s", "github.com/google/knative-gcp", localName)
			name = strings.Replace(name, "github.com/google/knative-gcp/vendor/", "", 1)
			parserMapCache[name] = v
		}
		fd, err := os.Open(current)
		if err != nil {
			return parserMapCache, fmt.Errorf("can't open: %w", err)
		}
		l, err := fd.Readdir(-1)
		if err != nil {
			return parserMapCache, fmt.Errorf("can't readdir: %w", err)
		}
		for _, f := range l {
			if f.IsDir() {
				pList = append(pList, fmt.Sprintf("%s/%s", current, f.Name()))
			}
		}
		err = fd.Close()
		if err != nil {
			return parserMapCache, fmt.Errorf("can't close: %w", err)
		}
	}
	return parserMapCache, nil
}

// parseFieldDocs parses the comments of a specific field. It attempts to figure out whether the
// comment says if this field is required or not.
func parseFieldDocs(f *ast.Field) (string, OpenAPIRequired) {
	if f.Doc == nil {
		return "", Unknown
	}
	var lines []string
	docSaysRequired := Unknown
	for _, line := range f.Doc.List {
		l := strings.TrimPrefix(line.Text, "//")
		l = strings.TrimSpace(l)
		skip := false
		switch strings.ToLower(l) {
		case "+optional":
			docSaysRequired = Optional
			continue
		case "+required":
			docSaysRequired = Required
			continue
		}
		if strings.HasPrefix(l, "+") {
			// Not really a comment, normally alters the semantics of the field, like mergePatchKey.
			continue
		}
		if strings.HasPrefix(l, "TODO") {
			// Assume that from this forward is a TODO, not real docs.
			skip = true
			break
		}
		if !skip {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, " "), docSaysRequired
}
