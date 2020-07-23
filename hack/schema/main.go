package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
)

var (
	trueVal = true
)

func main() {
	t := reflect.TypeOf(v1.Topic{})
	//t = reflect.TypeOf(v1alpha1.BrokerCell{})
	s := makeTopSchema(t)
	b, _ := yaml.Marshal(s)
	fmt.Print(string(b))
}

func makeTopSchema(t reflect.Type) JSONSchemaProps {
	s := makeSchemaStruct(t, true)
	return s
}

func makeSchema(t reflect.Type) (bool, JSONSchemaProps) {
	switch k := t.Kind(); k {
	case reflect.Bool:
		return true, JSONSchemaProps{
			Type: "boolean",
		}
	case reflect.Int:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "int32",
		}
	case reflect.Int8:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "int8",
		}
	case reflect.Int16:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "int16",
		}
	case reflect.Int32:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "int32",
		}
	case reflect.Int64:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "int64",
		}
	case reflect.Uint:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "uint32",
		}
	case reflect.Uint8:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "uint8",
		}
	case reflect.Uint16:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "uint16",
		}
	case reflect.Uint32:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "uint32",
		}
	case reflect.Uint64:
		return true, JSONSchemaProps{
			Type:   "integer",
			Format: "uint64",
		}
	case reflect.Uintptr:
		return false, JSONSchemaProps{
			Type:   "integer",
			Format: "uint32",
		}
	case reflect.Float32:
		return true, JSONSchemaProps{
			Type:   "float",
			Format: "float32",
		}
	case reflect.Float64:
		return true, JSONSchemaProps{
			Type:   "float",
			Format: "float64",
		}
	case reflect.Map:
		s := makeSchemaMap(t)
		return true, s
	case reflect.Ptr:
		_, s := makeSchema(t.Elem())
		return false, s
	case reflect.Slice:
		s := makeSchemaSlice(t)
		return true, s
	case reflect.String:
		return true, JSONSchemaProps{
			Type: "string",
		}
	case reflect.Struct:
		s := makeSchemaStruct(t, false)
		return true, s
	case reflect.Complex64:
		fallthrough
	case reflect.Complex128:
		fallthrough
	case reflect.Array:
		fallthrough
	case reflect.Chan:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Interface:
		fallthrough
	case reflect.UnsafePointer:
		panic(fmt.Errorf("can't handle kind %+v", t))
	default:
		panic(fmt.Errorf("unknown kind: %+v", t))
	}
}

var topLevelFieldsToSkip = map[string]struct{}{
	"TypeMeta":   {},
	"ObjectMeta": {},
}

func makeSchemaStruct(t reflect.Type, skipTopLevelCommon bool) JSONSchemaProps {
	s := JSONSchemaProps{
		Type:       "object",
		Properties: map[string]JSONSchemaProps{},
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if _, skip := topLevelFieldsToSkip[f.Name]; skip && skipTopLevelCommon {
			continue
		}
		name := f.Name
		jsonKey, present := f.Tag.Lookup("json")
		if present {
			split := strings.Split(jsonKey, ",")
			if split[0] != "" {
				name = split[0]
			}
		}

		ptrImpliesRequired, fs := makeSchema(f.Type)

		if selfJSONMarshaler(f.Type) {
			// The field marshals itself. Let's pretend it is a string.
			fs = JSONSchemaProps{
				Type: "string",
			}
		}

		if f.Anonymous {
			for n, p := range fs.Properties {
				s.Properties[n] = p
			}
			s.Required = append(s.Required, fs.Required...)
		} else {
			// Add docs
			doc, docSaysRequired, err := getDocs(t, f.Name)
			if err != nil {
				doc = fmt.Sprintf("not found: %v", err)
			}
			fs.Description = doc
			s.Properties[name] = fs
			switch docSaysRequired {
			case optional:
				// Nothing!
			case required:
				s.Required = append(s.Required, name)
			case unknown:
				// Doc didn't say anything, check if the type is a pointer.
				if ptrImpliesRequired {
					s.Required = append(s.Required, name)
				}
			}
		}

	}
	return s
}

func makeSchemaMap(t reflect.Type) JSONSchemaProps {
	if t.Key().Kind() != reflect.String {
		panic(fmt.Errorf("can't handle a non-string key: %+v", t))
	}
	return JSONSchemaProps{
		Type:                   "object",
		XPreserveUnknownFields: &trueVal,
	}
}

func makeSchemaSlice(t reflect.Type) JSONSchemaProps {
	s := JSONSchemaProps{
		Type: "array",
	}
	_, is := makeSchema(t.Elem())
	s.Items = &JSONSchemaPropsOrArray{
		Schema: &is,
	}
	return s
}

func selfJSONMarshaler(t reflect.Type) bool {
	jm := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	ju := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	if !t.Implements(jm) && !reflect.PtrTo(t).Implements(jm) {
		return false
	}
	return t.Implements(ju) || reflect.PtrTo(t).Implements(ju)
}

// JSONSchemaProps is a JSON-Schema following Specification Draft 4 (http://json-schema.org/).
type JSONSchemaProps struct {
	Description          string                     `yaml:"description,omitempty"`
	Type                 string                     `yaml:"type,omitempty"`
	Format               string                     `yaml:"format,omitempty"`
	Default              *JSON                      `yaml:",omitempty"`
	Maximum              *float64                   `yaml:",omitempty"`
	ExclusiveMaximum     bool                       `yaml:",omitempty"`
	Minimum              *float64                   `yaml:",omitempty"`
	ExclusiveMinimum     bool                       `yaml:",omitempty"`
	MaxLength            *int64                     `yaml:",omitempty"`
	MinLength            *int64                     `yaml:",omitempty"`
	Pattern              string                     `yaml:",omitempty"`
	MaxItems             *int64                     `yaml:",omitempty"`
	MinItems             *int64                     `yaml:",omitempty"`
	UniqueItems          bool                       `yaml:",omitempty"`
	MultipleOf           *float64                   `yaml:",omitempty"`
	Enum                 []JSON                     `yaml:",omitempty"`
	MaxProperties        *int64                     `yaml:",omitempty"`
	MinProperties        *int64                     `yaml:",omitempty"`
	Required             []string                   `yaml:",omitempty"`
	Items                *JSONSchemaPropsOrArray    `yaml:",omitempty"`
	AllOf                []JSONSchemaProps          `yaml:",omitempty"`
	OneOf                []JSONSchemaProps          `yaml:",omitempty"`
	AnyOf                []JSONSchemaProps          `yaml:",omitempty"`
	Not                  *JSONSchemaProps           `yaml:",omitempty"`
	Properties           map[string]JSONSchemaProps `yaml:",omitempty"`
	AdditionalProperties *JSONSchemaPropsOrBool     `yaml:",omitempty"`
	PatternProperties    map[string]JSONSchemaProps `yaml:",omitempty"`
	Dependencies         JSONSchemaDependencies     `yaml:",omitempty"`
	AdditionalItems      *JSONSchemaPropsOrBool     `yaml:",omitempty"`
	Definitions          JSONSchemaDefinitions      `yaml:",omitempty"`
	ExternalDocs         *ExternalDocumentation     `yaml:",omitempty"`
	Example              *JSON                      `yaml:",omitempty"`

	// x-kubernetes-preserve-unknown-fields stops the API server
	// decoding step from pruning fields which are not specified
	// in the validation schema. This affects fields recursively,
	// but switches back to normal pruning behaviour if nested
	// properties or additionalProperties are specified in the schema.
	// This can either be true or undefined. False is forbidden.
	XPreserveUnknownFields *bool `yaml:",omitempty"`

	// x-kubernetes-embedded-resource defines that the value is an
	// embedded Kubernetes runtime.Object, with TypeMeta and
	// ObjectMeta. The type must be object. It is allowed to further
	// restrict the embedded object. Both ObjectMeta and TypeMeta
	// are validated automatically. x-kubernetes-preserve-unknown-fields
	// must be true.
	XEmbeddedResource bool `yaml:",omitempty"`

	// x-kubernetes-int-or-string specifies that this value is
	// either an integer or a string. If this is true, an empty
	// type is allowed and type as child of anyOf is permitted
	// if following one of the following patterns:
	//
	// 1) anyOf:
	//    - type: integer
	//    - type: string
	// 2) allOf:
	//    - anyOf:
	//      - type: integer
	//      - type: string
	//    - ... zero or more
	XIntOrString bool `yaml:",omitempty"`

	// x-kubernetes-list-map-keys annotates an array with the x-kubernetes-list-type `map` by specifying the keys used
	// as the index of the map.
	//
	// This tag MUST only be used on lists that have the "x-kubernetes-list-type"
	// extension set to "map". Also, the values specified for this attribute must
	// be a scalar typed field of the child structure (no nesting is supported).
	XListMapKeys []string `yaml:",omitempty"`

	// x-kubernetes-list-type annotates an array to further describe its topology.
	// This extension must only be used on lists and may have 3 possible values:
	//
	// 1) `atomic`: the list is treated as a single entity, like a scalar.
	//      Atomic lists will be entirely replaced when updated. This extension
	//      may be used on any type of list (struct, scalar, ...).
	// 2) `set`:
	//      Sets are lists that must not have multiple items with the same value. Each
	//      value must be a scalar, an object with x-kubernetes-map-type `atomic` or an
	//      array with x-kubernetes-list-type `atomic`.
	// 3) `map`:
	//      These lists are like maps in that their elements have a non-index key
	//      used to identify them. Order is preserved upon merge. The map tag
	//      must only be used on a list with elements of type object.
	XListType *string `yaml:",omitempty"`

	// x-kubernetes-map-type annotates an object to further describe its topology.
	// This extension must only be used when type is object and may have 2 possible values:
	//
	// 1) `granular`:
	//      These maps are actual maps (key-value pairs) and each fields are independent
	//      from each other (they can each be manipulated by separate actors). This is
	//      the default behaviour for all maps.
	// 2) `atomic`: the list is treated as a single entity, like a scalar.
	//      Atomic maps will be entirely replaced when updated.
	// +optional
	XMapType *string `yaml:",omitempty"`
}

// JSON represents any valid JSON value.
// These types are supported: bool, int64, float64, string, []interface{}, map[string]interface{} and nil.
type JSON interface{}

// JSONSchemaURL represents a schema url.
type JSONSchemaURL string

// JSONSchemaPropsOrArray represents a value that can either be a JSONSchemaProps
// or an array of JSONSchemaProps. Mainly here for serialization purposes.
type JSONSchemaPropsOrArray struct {
	Schema      *JSONSchemaProps  `yaml:",inline,omitempty"`
	JSONSchemas []JSONSchemaProps `yaml:",omitempty"`
}

// JSONSchemaPropsOrBool represents JSONSchemaProps or a boolean value.
// Defaults to true for the boolean property.
type JSONSchemaPropsOrBool struct {
	Allows bool             `yaml:",omitempty"`
	Schema *JSONSchemaProps `yaml:",omitempty"`
}

// JSONSchemaDependencies represent a dependencies property.
type JSONSchemaDependencies map[string]JSONSchemaPropsOrStringArray

// JSONSchemaPropsOrStringArray represents a JSONSchemaProps or a string array.
type JSONSchemaPropsOrStringArray struct {
	Schema   *JSONSchemaProps `yaml:",omitempty"`
	Property []string         `yaml:",omitempty"`
}

// JSONSchemaDefinitions contains the models explicitly defined in this spec.
type JSONSchemaDefinitions map[string]JSONSchemaProps

// ExternalDocumentation allows referencing an external resource for extended documentation.
type ExternalDocumentation struct {
	Description string
	URL         string
}

// Docs

func ignoreDirectories(fi os.FileInfo) bool {
	return !fi.IsDir()
}

var parserMapCache = map[string]*ast.Package{}

func makeParserMapForPackage(pkg string) (map[string]*ast.Package, error) {
	fs := token.NewFileSet()
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

type openAPIRequired int

const (
	unknown openAPIRequired = iota
	optional
	required
)

func getDocs(t reflect.Type, fieldName string) (string, openAPIRequired, error) {
	p, err := makeParserMapForPackage(t.PkgPath())
	pkg := t.PkgPath()
	//fs := token.NewFileSet()
	//p, err := parser.ParseDir(fs, pkg, nil, parser.ParseComments)
	if err != nil {
		return "", unknown, fmt.Errorf("unable to parse dir: %w", err)
	}
	ap, present := p[pkg]
	if !present {
		return "", unknown, fmt.Errorf("package not present: %q", pkg)
	}
	dp := doc.New(ap, pkg, 0)
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
	return "", unknown, fmt.Errorf("did not find doc for %q", t.Name())
}

func parseFieldDocs(f *ast.Field) (string, openAPIRequired) {
	if f.Doc == nil {
		return "", unknown
	}
	var lines []string
	docSaysRequired := unknown
	for _, line := range f.Doc.List {
		l := strings.TrimPrefix(line.Text, "// ")
		l = strings.TrimSpace(l)
		skip := false
		switch strings.ToLower(l) {
		case "+optional":
			docSaysRequired = optional
			continue
		case "+required":
			docSaysRequired = required
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
	return strings.Join(lines, "\n"), docSaysRequired
}
