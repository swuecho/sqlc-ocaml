package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hwu/sqlc-ocaml/internal/plugin"
)

// Program is the normalized, protocol-independent view consumed by the OCaml
// renderer. Protocol details and PostgreSQL type resolution stop here.
type Program struct {
	Enums   []Enum
	Models  []Record
	Queries []Query
}

type Enum struct {
	DatabaseName string
	TypeName     string
	CodecName    string
	Values       []EnumValue
}

type EnumValue struct {
	DatabaseName string
	Constructor  string
}

type Cardinality uint8

const (
	Exec Cardinality = iota
	ExecRows
	One
	Many
)

type Query struct {
	SourceName  string
	SourceFile  string
	ModuleName  string
	SQL         string
	Cardinality Cardinality
	Params      Record
	Bindings    []ParameterBinding
	Row         *Record
}

// ParameterBinding identifies the logical parameter field used by one SQL
// placeholder occurrence. Bindings are ordered as Caqti sees the question
// marks, so repeated and reordered PostgreSQL placeholders remain type-safe.
type ParameterBinding struct {
	FieldIndex int
}

type Record struct {
	TypeName string
	Fields   []Field
}

type Field struct {
	DatabaseName string
	Name         string
	Type         OCamlType
	Embedded     *Record
}

type OCamlType struct {
	DatabaseType string
	Name         string
	Codec        string
	Nullable     bool
	Element      *OCamlType
}

func normalizedOCamlType(column *plugin.Column, mapped mappedType) OCamlType {
	t := OCamlType{DatabaseType: dbType(column), Name: mapped.OCaml, Codec: mapped.Codec, Nullable: !column.NotNull}
	if column.IsArray || column.ArrayDims == 1 {
		elementName := strings.TrimSuffix(strings.TrimSuffix(mapped.OCaml, " option"), " list")
		t.Element = &OCamlType{DatabaseType: dbType(column), Name: elementName}
	}
	return t
}

func (g *gen) normalize() (Program, error) {
	program := Program{}
	if g.models == nil {
		g.models = map[string]Record{}
	}
	keys := make([]string, 0, len(g.enums))
	for key := range g.enums {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		e := g.enums[key]
		item := Enum{DatabaseName: e.Name, TypeName: snake(e.Name), CodecName: snake(e.Name) + "_type"}
		for _, value := range e.Vals {
			item.Values = append(item.Values, EnumValue{DatabaseName: value, Constructor: constructor(value)})
		}
		program.Enums = append(program.Enums, item)
	}
	if g.req.Catalog != nil {
		for _, schema := range g.req.Catalog.Schemas {
			for _, table := range schema.Tables {
				if table == nil || table.Rel == nil || table.Rel.Schema == "pg_catalog" || table.Rel.Schema == "information_schema" {
					continue
				}
				model, err := g.normalizeRecord(snake(table.Rel.Name), table.Columns)
				if err != nil {
					return Program{}, fmt.Errorf("table %s: %w", table.Rel.Name, err)
				}
				if _, exists := g.models[table.Rel.Name]; exists {
					return Program{}, fmt.Errorf("duplicate table model name %q", table.Rel.Name)
				}
				g.models[table.Rel.Name] = model
				program.Models = append(program.Models, model)
			}
		}
	}

	seen := map[string]bool{}
	for _, source := range g.req.Queries {
		query, err := g.normalizeQuery(source)
		if err != nil {
			return Program{}, fmt.Errorf("query %s: %w", source.Name, err)
		}
		if seen[query.ModuleName] {
			return Program{}, fmt.Errorf("duplicate OCaml module name %q", query.ModuleName)
		}
		seen[query.ModuleName] = true
		program.Queries = append(program.Queries, query)
	}
	return program, nil
}

func (g *gen) normalizeQuery(source *plugin.Query) (Query, error) {
	cardinality, err := normalizeCardinality(source.Cmd)
	if err != nil {
		return Query{}, err
	}
	queryParams := source.AllParams()
	paramColumns := make([]*plugin.Column, len(queryParams))
	fieldByNumber := make(map[int]int, len(queryParams))
	for i, param := range queryParams {
		if param == nil || param.Column == nil {
			return Query{}, fmt.Errorf("parameter %d has no column metadata", i+1)
		}
		number := int(param.Number)
		if number < 1 || number > len(queryParams) {
			return Query{}, fmt.Errorf("parameter %d has invalid number %d", i+1, number)
		}
		if _, exists := fieldByNumber[number]; exists {
			return Query{}, fmt.Errorf("duplicate parameter number $%d", number)
		}
		fieldByNumber[number] = i
		paramColumns[i] = param.Column
	}
	sql, occurrences, err := caqtiSQL(source.Text, len(paramColumns))
	if err != nil {
		return Query{}, err
	}
	params, err := g.normalizeRecord("params", paramColumns)
	if err != nil {
		return Query{}, err
	}
	bindings := make([]ParameterBinding, len(occurrences))
	for i, number := range occurrences {
		fieldIndex, ok := fieldByNumber[number]
		if !ok {
			return Query{}, fmt.Errorf("SQL placeholder $%d has no parameter metadata", number)
		}
		bindings[i] = ParameterBinding{FieldIndex: fieldIndex}
	}
	query := Query{SourceName: source.Name, SourceFile: source.Filename, ModuleName: constructor(source.Name), SQL: sql, Cardinality: cardinality, Params: params, Bindings: bindings}
	if cardinality != Exec && cardinality != ExecRows {
		row, err := g.normalizeRow(source.Columns)
		if err != nil {
			return Query{}, err
		}
		query.Row = &row
	}
	return query, nil
}

func (g *gen) normalizeRow(columns []*plugin.Column) (Record, error) {
	names, err := uniqueFields(columns)
	if err != nil {
		return Record{}, err
	}
	row := Record{TypeName: "row", Fields: make([]Field, len(columns))}
	for i, column := range columns {
		if column == nil {
			return Record{}, fmt.Errorf("field %d has no column metadata", i+1)
		}
		if column.EmbedTable == nil {
			mapped, err := g.mapType(column)
			if err != nil {
				return Record{}, err
			}
			row.Fields[i] = Field{DatabaseName: column.Name, Name: names[i], Type: normalizedOCamlType(column, mapped)}
			continue
		}
		model, ok := g.models[column.EmbedTable.Name]
		if !ok {
			return Record{}, fmt.Errorf("embedded table %q has no model metadata", column.EmbedTable.Name)
		}
		modelCopy := model
		row.Fields[i] = Field{DatabaseName: column.Name, Name: names[i], Type: OCamlType{Name: model.TypeName}, Embedded: &modelCopy}
	}
	return row, nil
}

func normalizeCardinality(command string) (Cardinality, error) {
	switch strings.TrimPrefix(command, ":") {
	case "exec":
		return Exec, nil
	case "execrows":
		return ExecRows, nil
	case "one":
		return One, nil
	case "many":
		return Many, nil
	default:
		return 0, fmt.Errorf("unsupported command %q (supported: :one, :many, :exec, :execrows)", command)
	}
}

func (g *gen) normalizeRecord(typeName string, columns []*plugin.Column) (Record, error) {
	names, err := uniqueFields(columns)
	if err != nil {
		return Record{}, err
	}
	record := Record{TypeName: typeName, Fields: make([]Field, len(columns))}
	for i, column := range columns {
		if column == nil {
			return Record{}, fmt.Errorf("field %d has no column metadata", i+1)
		}
		mapped, err := g.mapType(column)
		if err != nil {
			return Record{}, err
		}
		record.Fields[i] = Field{DatabaseName: column.Name, Name: names[i], Type: normalizedOCamlType(column, mapped)}
	}
	return record, nil
}
