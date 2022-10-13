package types

import (
	"fmt"
	"strings"
)

type colType int

const (
	integer colType = iota
	boolean
	bigint
	numeric
	bytea
	varchar
	text
)

type column struct {
	name string
	typ  colType
}
type Table struct {
	Name           string
	Columns        []column
	ConflictClause string
}

func (tbl *Table) ToCsvRow(args ...interface{}) []string {
	var row []string
	for i, col := range tbl.Columns {
		row = append(row, col.typ.formatter()(args[i]))
	}
	return row
}

func (tbl *Table) ToInsertStatement() string {
	var colnames, placeholders []string
	for i, col := range tbl.Columns {
		colnames = append(colnames, col.name)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) %s",
		tbl.Name, strings.Join(colnames, ", "), strings.Join(placeholders, ", "), tbl.ConflictClause,
	)
}

type colfmt = func(interface{}) string

func sprintf(f string) colfmt {
	return func(x interface{}) string { return fmt.Sprintf(f, x) }
}

func (typ colType) formatter() colfmt {
	switch typ {
	case integer:
		return sprintf("%d")
	case boolean:
		return func(x interface{}) string {
			if x.(bool) {
				return "t"
			}
			return "f"
		}
	case bigint:
		return sprintf("%s")
	case numeric:
		return sprintf("%d")
	case bytea:
		return sprintf(`\x%x`)
	case varchar:
		return sprintf("%s")
	case text:
		return sprintf("%s")
	}
	panic("unreachable")
}
