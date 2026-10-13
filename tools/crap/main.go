// Command crap enforces the per-function CRAP ceiling:
//
//	complexity² × (1 - coverage)³ + complexity
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type key struct {
	file string
	line int
}

type function struct {
	file       string
	line       int
	name       string
	complexity int
	coverage   float64
	score      float64
}

func main() {
	profile := flag.String("profile", "", "Go coverage profile")
	maximum := flag.Float64("max", 15, "maximum permitted CRAP score")
	top := flag.Int("top", 15, "number of worst functions to print")
	flag.Parse()
	if *profile == "" {
		fatal("usage: crap -profile <file> -max <score>")
	}
	root, err := filepath.Abs(".")
	if err != nil {
		fatal("resolve root: %v", err)
	}
	functions, err := scan(root)
	if err != nil {
		fatal("scan complexity: %v", err)
	}
	module, err := modulePath(root)
	if err != nil {
		fatal("module path: %v", err)
	}
	coverage, err := readCoverage(*profile, module)
	if err != nil {
		fatal("coverage: %v", err)
	}
	joined := 0
	results := make([]function, 0, len(functions))
	for identifier, entry := range functions {
		measured, exists := coverage[identifier]
		if !exists {
			continue
		}
		entry.coverage = measured
		joined++
		uncovered := 1 - entry.coverage
		entry.score = math.Pow(float64(entry.complexity), 2)*math.Pow(uncovered, 3) + float64(entry.complexity)
		results = append(results, entry)
	}
	if joined == 0 {
		fatal("coverage profile did not match any scanned function")
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].file+results[i].name < results[j].file+results[j].name
	})
	over := 0
	for _, result := range results {
		if result.score > *maximum {
			over++
		}
	}
	limit := *top
	if limit > len(results) {
		limit = len(results)
	}
	fmt.Printf("%d Go functions, %d over CRAP %.0f\n", len(results), over, *maximum)
	for _, result := range results[:limit] {
		fmt.Printf("%5.1f  cplx=%-2d cov=%5.1f%%  %s:%d %s\n",
			result.score,
			result.complexity,
			result.coverage*100,
			result.file,
			result.line,
			result.name,
		)
	}
	if over > 0 {
		fatal("%d function(s) exceed CRAP %.0f", over, *maximum)
	}
}

func scan(root string) (map[key]function, error) {
	found := make(map[key]function)
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "build", "dist", "graphify-out", "node_modules", "test", "tests", "tools":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range parsed.Decls {
			declared, ok := declaration.(*ast.FuncDecl)
			if !ok || declared.Body == nil {
				continue
			}
			position := fileSet.Position(declared.Pos())
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			found[key{file: relative, line: position.Line}] = function{
				file: relative, line: position.Line, name: functionName(declared), complexity: complexity(declared),
			}
		}
		return nil
	})
	return found, err
}

func complexity(declared *ast.FuncDecl) int {
	value := 1
	ast.Inspect(declared, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			value++
		case *ast.BinaryExpr:
			if typed.Op == token.LAND || typed.Op == token.LOR {
				value++
			}
		}
		return true
	})
	return value
}

func functionName(declared *ast.FuncDecl) string {
	if declared.Recv == nil || len(declared.Recv.List) == 0 {
		return declared.Name.Name
	}
	name := "?"
	switch receiver := declared.Recv.List[0].Type.(type) {
	case *ast.Ident:
		name = receiver.Name
	case *ast.StarExpr:
		if identifier, ok := receiver.X.(*ast.Ident); ok {
			name = "*" + identifier.Name
		}
	}
	return "(" + name + ")." + declared.Name.Name
}

func readCoverage(profile, module string) (map[key]float64, error) {
	output, err := exec.Command("go", "tool", "cover", "-func="+profile).Output()
	if err != nil {
		return nil, err
	}
	found := make(map[key]float64)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[0] == "total:" {
			continue
		}
		location := strings.Split(fields[0], ":")
		if len(location) < 3 {
			continue
		}
		line, err := strconv.Atoi(location[len(location)-2])
		if err != nil {
			continue
		}
		percent, err := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		if err != nil {
			continue
		}
		path := strings.Join(location[:len(location)-2], ":")
		path = strings.TrimPrefix(strings.TrimPrefix(path, module), "/")
		found[key{file: path, line: line}] = percent / 100
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, errors.New("profile names no functions")
	}
	return found, nil
}

func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if module, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.TrimSpace(module), nil
		}
	}
	return "", errors.New("go.mod has no module directive")
}

func fatal(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
