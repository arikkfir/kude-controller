package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"golang.org/x/tools/go/packages"
	"html/template"
	"log"
	"os"
)

const (
	k8sMetaV1PackageName    = "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sMetaV1TypeMetaType   = "TypeMeta"
	k8sMetaV1ObjectMetaType = "ObjectMeta"
	TypeMetaFieldName       = "TypeMeta"
	ObjectMetaFieldName     = "ObjectMeta"
	StatusFieldName         = "Status"
)

var (
	//go:embed object.go.tmpl
	tmplText string
)

func main() {
	// Configure logging
	log.SetFlags(0)
	log.SetPrefix("")

	// Parse template
	tmpl, err := template.New("").Parse(tmplText)
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	// Parse & validate flags
	var typeName string
	flag.StringVar(&typeName, "type", "", "type name")
	flag.Parse()
	if len(flag.Args()) != 0 {
		log.Fatal("arguments not allowed")
	}

	// Discover packages from current path
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		log.Fatal(err)
	} else if len(pkgs) != 1 {
		log.Fatalf("Expected 1 package, found %d", len(pkgs))
	}

	// Load package
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		for _, err := range pkg.Errors {
			log.Printf("Error loading package: %v", err)
		}
		os.Exit(1)
	}

	typeInfo := pkg.Types.Scope().Lookup(typeName)
	if typeInfo == nil {
		log.Fatalf("Could not find type '%s'", typeName)
	} else if !typeInfo.Exported() {
		log.Fatal("Type is not exported")
	}

	namedType, ok := typeInfo.Type().(*types.Named)
	if !ok {
		log.Fatal("Type is not named")
	}

	structType, ok := namedType.Underlying().(*types.Struct)
	if !ok {
		log.Fatal("Type is not a struct")
	}

	var (
		TypeMetaFound   = false
		ObjectMetaFound = false
		StatusFound     = false
	)
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		if field.Name() == TypeMetaFieldName {
			t := field.Type()
			namedType, ok := t.(*types.Named)
			if !ok {
				log.Fatalf("Expected field '%s' type to be named", field.Name())
			}

			obj := namedType.Obj()
			if obj.Pkg().String() != fmt.Sprintf("package v1 (\"%s\")", k8sMetaV1PackageName) || obj.Name() != k8sMetaV1TypeMetaType {
				log.Fatalf("Expected field '%s' type to be '%s.%s'", field.Name(), k8sMetaV1PackageName, k8sMetaV1TypeMetaType)
			}
			TypeMetaFound = true
		} else if field.Name() == ObjectMetaFieldName {
			t := field.Type()
			namedType, ok := t.(*types.Named)
			if !ok {
				log.Fatalf("Expected field '%s' type to be named", field.Name())
			}

			obj := namedType.Obj()
			if obj.Pkg().String() != fmt.Sprintf("package v1 (\"%s\")", k8sMetaV1PackageName) || obj.Name() != k8sMetaV1ObjectMetaType {
				log.Fatalf("Expected field '%s' type to be '%s.%s'", field.Name(), k8sMetaV1PackageName, k8sMetaV1ObjectMetaType)
			}
			ObjectMetaFound = true
		} else if field.Name() == StatusFieldName {
			// TODO: verify status field
			StatusFound = true
		}
	}
	if !TypeMetaFound {
		log.Fatalf("Type does not have '%s' field", TypeMetaFieldName)
	} else if !ObjectMetaFound {
		log.Fatalf("Type does not have '%s' field", ObjectMetaFieldName)
	} else if !StatusFound {
		log.Fatalf("Type does not have '%s' field", StatusFieldName)
	}

	// Generate code
	templateData := map[string]interface{}{
		"PackageName": pkg.Name,
		"StructName":  typeName,
	}
	var processed bytes.Buffer
	if err := tmpl.Execute(&processed, templateData); err != nil {
		log.Fatalf("Error generating code: %v", err)
	}

	// Format
	formatted, err := format.Source(processed.Bytes())
	if err != nil {
		log.Fatalf("Could not format processed template: %v\n%s", err, processed.String())
	}

	// Write
	dstFile := typeInfo.Name() + "_object.go"
	if err := os.WriteFile(dstFile, formatted, 0644); err != nil {
		log.Fatalf("Failed writing code to '%s': %v", dstFile, err)
	}
}
