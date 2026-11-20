package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

var (
	kind       = flag.String("kind", "resource", "resource | data_source")
	name       = flag.String("name", "", "スネークケースのリソース名 (例: service_label)")
	pkg        = flag.String("pkg", "provider", "出力先のGoパッケージ名（ディレクトリ名と一致させる）")
	outDir     = flag.String("out", "", "出力ディレクトリ（未指定なら ./<pkg>）")
	clientPkg  = flag.String("client-pkg", "github.com/topotal/waroom-client-go/waroom", "クライアントのimportパス")
	clientType = flag.String("client-type", "Client", "クライアントの型名（ポインタ前提）")
)

// テンプレート本体。
//
//go:embed resource.tmpl
var resourceTmpl string

//go:embed data_source.tmpl
var dataSourceTmpl string

// -kind ごとのテンプレート。
var templates = map[string]string{
	"resource":    resourceTmpl,
	"data_source": dataSourceTmpl,
}

func main() {
	flag.Parse()

	if *name == "" {
		exitf("missing -name (snake_case e.g. service_label)")
	}
	tmpl, ok := templates[*kind]
	if !ok {
		exitf("unsupported -kind: %s (resource | data_source)", *kind)
	}

	snake := *name
	pascal := toPascal(snake) // e.g. ServiceLabel
	camel := toCamel(snake)   // e.g. serviceLabel

	out := *outDir
	if out == "" {
		out = "./" + *pkg
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		exitf("mkdir: %v", err)
	}

	filename := filepath.Join(out, fmt.Sprintf("%s_%s.go", *kind, snake)) // resource_service_label.go / data_source_service_label.go
	if exists(filename) {
		exitf("file already exists: %s", filename)
	}

	data := map[string]any{
		"Package":      *pkg,
		"Snake":        snake,
		"Pascal":       pascal,      // ServiceLabel
		"Camel":        camel,       // serviceLabel
		"ClientImport": *clientPkg,  // github.com/topotal/waroom-client-go/waroom
		"ClientType":   *clientType, // Client
	}

	tpl, err := template.New(*kind).Parse(tmpl)
	if err != nil {
		exitf("template parse: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		exitf("template exec: %v", err)
	}

	if err := os.WriteFile(filename, buf.Bytes(), 0o644); err != nil {
		exitf("write file: %v", err)
	}

	fmt.Println("generated:", filename)
}

func exitf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func toPascal(s string) string {
	s = strings.Trim(nonAlphaNum.ReplaceAllString(s, "_"), "_")
	parts := strings.Split(s, "_")
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, "")
}

func toCamel(s string) string {
	p := toPascal(s)
	if p == "" {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
