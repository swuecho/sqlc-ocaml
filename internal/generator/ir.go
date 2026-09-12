package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/swuecho/sqlc-ocaml/internal/plugin"
)

// Program is the normalized, protocol-independent view consumed by the OCaml
// renderer. Protocol details and PostgreSQL type resolution stop here.
type Program struct {
	Enums      []Enum
	Models     []Record
	SharedRows []Record
	Queries    []Query
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
	SharedRow   string
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
	Table        string
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
	typeNames := map[string]string{}
	if g.models == nil {
		g.models = map[string]Record{}
	}
	enums := append([]enumInfo(nil), g.enumList...)
	sort.Slice(enums, func(i, j int) bool {
		return qualifiedName("", enums[i].Schema, enums[i].Enum.Name) < qualifiedName("", enums[j].Schema, enums[j].Enum.Name)
	})
	for _, info := range enums {
		e := info.Enum
		item := Enum{DatabaseName: e.Name, TypeName: info.TypeName, CodecName: info.TypeName + "_type"}
		if previous, exists := typeNames[item.TypeName]; exists {
			return Program{}, fmt.Errorf("OCaml type name %q is generated for both %s and enum %s.%s", item.TypeName, previous, info.Schema, e.Name)
		}
		typeNames[item.TypeName] = fmt.Sprintf("enum %s.%s", info.Schema, e.Name)
		constructors := make(map[string]string, len(e.Vals))
		for _, value := range e.Vals {
			name := constructor(value)
			if previous, exists := constructors[name]; exists {
				return Program{}, fmt.Errorf("enum %s.%s values %q and %q both generate OCaml constructor %q", info.Schema, e.Name, previous, value, name)
			}
			constructors[name] = value
			item.Values = append(item.Values, EnumValue{DatabaseName: value, Constructor: name})
		}
		program.Enums = append(program.Enums, item)
	}
	if g.req.Catalog != nil {
		tableCounts := map[string]int{}
		for _, schema := range g.req.Catalog.Schemas {
			for _, table := range schema.Tables {
				if table != nil && table.Rel != nil {
					tableCounts[strings.ToLower(table.Rel.Name)]++
				}
			}
		}
		for _, schema := range g.req.Catalog.Schemas {
			for _, table := range schema.Tables {
				if table == nil || table.Rel == nil || table.Rel.Schema == "pg_catalog" || table.Rel.Schema == "information_schema" {
					continue
				}
				typeName := snake(table.Rel.Name)
				if tableCounts[strings.ToLower(table.Rel.Name)] > 1 {
					typeName = snake(schema.Name + "_" + table.Rel.Name)
				}
				objectName := fmt.Sprintf("table %s.%s", schema.Name, table.Rel.Name)
				if previous, exists := typeNames[typeName]; exists {
					return Program{}, fmt.Errorf("OCaml type name %q is generated for both %s and %s", typeName, previous, objectName)
				}
				typeNames[typeName] = objectName
				model, err := g.normalizeRecord(typeName, table.Columns)
				if err != nil {
					return Program{}, fmt.Errorf("table %s: %w", table.Rel.Name, err)
				}
				key := qualifiedName(table.Rel.Catalog, table.Rel.Schema, table.Rel.Name)
				if table.Rel.Schema == "" {
					key = qualifiedName(table.Rel.Catalog, schema.Name, table.Rel.Name)
				}
				if _, exists := g.models[key]; exists {
					return Program{}, fmt.Errorf("duplicate table model name %q", table.Rel.Name)
				}
				g.models[key] = model
				if tableCounts[strings.ToLower(table.Rel.Name)] == 1 {
					g.models[strings.ToLower(table.Rel.Name)] = model
				}
				program.Models = append(program.Models, model)
			}
		}
	}

	seen := map[string]bool{}
	for i, source := range g.req.Queries {
		if source == nil {
			return Program{}, fmt.Errorf("query %d is null", i+1)
		}
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
	tableColumns := map[string]map[string]bool{}
	if g.req.Catalog != nil {
		for _, schema := range g.req.Catalog.Schemas {
			for _, table := range schema.Tables {
				if table == nil || table.Rel == nil {
					continue
				}
				columns := map[string]bool{}
				for _, column := range table.Columns {
					if column != nil {
						columns[strings.ToLower(column.Name)] = true
					}
				}
				tableColumns[strings.ToLower(table.Rel.Name)] = columns
			}
		}
	}
	shareIdenticalRows(&program, g.req.Queries, typeNames, tableColumns)
	return program, nil
}

// shareIdenticalRows hoists result records used by multiple queries. OCaml
// records are nominal, so aliases to a shared top-level record let callers use
// one function for queries returning the same projection.
func shareIdenticalRows(program *Program, sources []*plugin.Query, typeNames map[string]string, tableColumns map[string]map[string]bool) {
	groups := map[string][]int{}
	for i, query := range program.Queries {
		if query.Row != nil {
			groups[recordShape(*query.Row)] = append(groups[recordShape(*query.Row)], i)
		}
	}
	seenShapes := map[string]bool{}
	for i, query := range program.Queries {
		if query.Row == nil {
			continue
		}
		shape := recordShape(*query.Row)
		if seenShapes[shape] {
			continue
		}
		seenShapes[shape] = true
		indexes := groups[shape]
		if len(indexes) < 2 {
			continue
		}
		first := i
		name := sharedRowName(sources[first], program.Queries[first], tableColumns)
		base := name
		for suffix := 2; typeNames[name] != ""; suffix++ {
			name = fmt.Sprintf("%s_%d", base, suffix)
		}
		typeNames[name] = "shared query row"
		shared := *program.Queries[first].Row
		shared.TypeName = name
		program.SharedRows = append(program.SharedRows, shared)
		for _, index := range indexes {
			program.Queries[index].SharedRow = name
		}
	}
}

func recordShape(record Record) string {
	var b strings.Builder
	for _, field := range record.Fields {
		fmt.Fprintf(&b, "%s|%d:%s%d:%s;", strings.ToLower(field.Table), len(field.Name), field.Name, len(field.Type.Name), field.Type.Name)
	}
	return b.String()
}

func sharedRowName(source *plugin.Query, query Query, tableColumns map[string]map[string]bool) string {
	// Use the table name only for a projection of exactly that table's columns.
	// Partial projections and aggregates fall back to the first query's name so
	// the type is not mislabelled after the table.
	if table := commonTable(source); table != "" {
		if columns := tableColumns[strings.ToLower(table)]; columns != nil && fullTableProjection(source, columns) {
			return singularize(snake(table)) + "_row"
		}
	}
	return snake(query.SourceName) + "_row"
}

// commonTable returns the single table every projected column comes from, or ""
// when the projection spans tables or a column has no table.
func commonTable(source *plugin.Query) string {
	table := ""
	for _, column := range source.Columns {
		if column == nil || column.Table == nil || column.Table.Name == "" {
			return ""
		}
		if table == "" {
			table = column.Table.Name
		} else if !strings.EqualFold(table, column.Table.Name) {
			return ""
		}
	}
	return table
}

// fullTableProjection reports whether the query selects every column of the
// table (aliases included, matched via the original column name).
func fullTableProjection(source *plugin.Query, columns map[string]bool) bool {
	seen := map[string]bool{}
	for _, column := range source.Columns {
		if column == nil {
			return false
		}
		name := column.OriginalName
		if name == "" {
			name = column.Name
		}
		if !columns[strings.ToLower(name)] {
			return false
		}
		seen[strings.ToLower(name)] = true
	}
	return len(seen) == len(columns)
}

var irregularPlurals = map[string]string{
	"people": "person",
	"children": "child",
	"men": "man",
	"women": "woman",
	"teeth": "tooth",
	"feet": "foot",
	"mice": "mouse",
	"geese": "goose",
}

// singularize turns a table name into an OCaml-friendly singular form. It only
// handles the regular English cases plus a few common irregulars; unknown names
// are returned unchanged.
func singularize(name string) string {
	if singular, ok := irregularPlurals[name]; ok {
		return singular
	}
	switch {
	case strings.HasSuffix(name, "ies") && len(name) > 3:
		return name[:len(name)-3] + "y"
	case strings.HasSuffix(name, "sses"),
		strings.HasSuffix(name, "shes"),
		strings.HasSuffix(name, "ches"),
		strings.HasSuffix(name, "xes"),
		strings.HasSuffix(name, "zes"):
		return name[:len(name)-2]
	case strings.HasSuffix(name, "s"),
		!strings.HasSuffix(name, "ss"),
		!strings.HasSuffix(name, "us"),
		!strings.HasSuffix(name, "is"):
		return name[:len(name)-1]
	default:
		return name
	}
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
	query := Query{SourceName: source.Name, SourceFile: source.Filename, ModuleName: moduleName(source.Name), SQL: sql, Cardinality: cardinality, Params: params, Bindings: bindings}
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
		model, ok := g.models[qualifiedName(column.EmbedTable.Catalog, column.EmbedTable.Schema, column.EmbedTable.Name)]
		if !ok {
			model, ok = g.models[strings.ToLower(column.EmbedTable.Name)]
		}
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
		table := ""
		if column.Table != nil {
			table = column.Table.Name
		}
		record.Fields[i] = Field{DatabaseName: column.Name, Table: table, Name: names[i], Type: normalizedOCamlType(column, mapped)}
	}
	return record, nil
}
