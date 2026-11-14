package main

import (
	"errors"
	"os"
	"path/filepath"
)

type NumberCheck struct{}

func (check NumberCheck) Repository(arguments []string) ([]Problem, error) {
	root, err := check.root(arguments)
	if err != nil {
		return nil, err
	}
	table, err := check.table(root)
	if err != nil {
		return nil, err
	}
	unconverted, err := NameList{}.Read(filepath.Join(root, "tools", "controllers", "unconverted.list"))
	if err != nil {
		return nil, err
	}
	used, err := check.used(root, table.Owners.Converted(unconverted), table.Owners.Kit())
	return append(table.Problems(), CodeComparison{Table: table, Used: used}.Problems()...), err
}

func (NumberCheck) root(arguments []string) (string, error) {
	if len(arguments) != 1 {
		return "", errors.New("give one argument: the repository folder")
	}
	return arguments[0], nil
}

func (NumberCheck) table(root string) (ErrorsTable, error) {
	text, err := os.ReadFile(filepath.Join(root, "docs", "mesail", "ERRORS.md"))
	return ErrorsTable{}.Parse(string(text)), err
}

func (NumberCheck) used(root string, owners Owners, kit Owner) ([]OwnerNumbers, error) {
	used := []OwnerNumbers{}
	for _, owner := range owners {
		numbers, err := OwnerScan{Root: root, Owner: owner, Kit: kit}.Numbers()
		if err != nil {
			return nil, err
		}
		used = append(used, OwnerNumbers{Owner: owner, Numbers: numbers})
	}
	return used, nil
}
