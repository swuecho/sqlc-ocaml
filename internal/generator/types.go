package generator

import (
	"fmt"
	"strings"

	"github.com/hwu/sqlc-ocaml/internal/plugin"
)

type mappedType struct{ OCaml, Codec string }

var postgresTypes = map[string]mappedType{
	"bool": {"bool", "Caqti_type.bool"}, "boolean": {"bool", "Caqti_type.bool"},
	"int2": {"int", "Caqti_type.int"}, "smallint": {"int", "Caqti_type.int"},
	"int4": {"int32", "Caqti_type.int32"}, "integer": {"int32", "Caqti_type.int32"}, "serial": {"int32", "Caqti_type.int32"},
	"int8": {"int64", "Caqti_type.int64"}, "bigint": {"int64", "Caqti_type.int64"}, "bigserial": {"int64", "Caqti_type.int64"},
	"float4": {"float", "Caqti_type.float"}, "real": {"float", "Caqti_type.float"},
	"float8": {"float", "Caqti_type.float"}, "double precision": {"float", "Caqti_type.float"},
	"text": {"string", "Caqti_type.string"}, "varchar": {"string", "Caqti_type.string"}, "character varying": {"string", "Caqti_type.string"},
	"char": {"string", "Caqti_type.string"}, "bpchar": {"string", "Caqti_type.string"}, "character": {"string", "Caqti_type.string"},
	"bytea":     {"string", "Caqti_type.octets"},
	"uuid":      {"Uuidm.t", "Caqti_type.uuid"},
	"date":      {"Ptime.date", "Caqti_type.date"},
	"timestamp": {"Ptime.t", "Caqti_type.ptime"}, "timestamp without time zone": {"Ptime.t", "Caqti_type.ptime"},
	"timestamptz": {"Ptime.t", "Caqti_type.ptime"}, "timestamp with time zone": {"Ptime.t", "Caqti_type.ptime"},
	"json":    {"Yojson.Safe.t", "Caqti_type.(custom ~encode:(fun x -> Ok (Yojson.Safe.to_string x)) ~decode:(fun x -> try Ok (Yojson.Safe.from_string x) with Yojson.Json_error e -> Error e) string)"},
	"jsonb":   {"Yojson.Safe.t", "Caqti_type.(custom ~encode:(fun x -> Ok (Yojson.Safe.to_string x)) ~decode:(fun x -> try Ok (Yojson.Safe.from_string x) with Yojson.Json_error e -> Error e) string)"},
	"numeric": {"string", "Caqti_type.string"}, "decimal": {"string", "Caqti_type.string"},
}

func dbType(col *plugin.Column) string {
	if col == nil || col.Type == nil {
		return ""
	}
	return normalizedDBType(col.Type.Name)
}

func normalizedDBType(value string) string {
	name := strings.ToLower(value)
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	return name
}

func qualifiedName(catalog, schema, name string) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{catalog, schema, name} {
		if part != "" {
			parts = append(parts, strings.ToLower(part))
		}
	}
	return strings.Join(parts, ".")
}

func (g *gen) enumFor(col *plugin.Column) (enumInfo, bool) {
	if col != nil && col.Type != nil {
		if info, ok := g.enums[qualifiedName(col.Type.Catalog, col.Type.Schema, col.Type.Name)]; ok {
			return info, true
		}
	}
	info, ok := g.enums[dbType(col)]
	return info, ok
}

func (g *gen) matchingOverride(col *plugin.Column, dbType string) (mappedType, bool) {
	nullable := !col.NotNull
	bestScore := -1
	var best mappedType
	for _, rule := range g.overrideRules {
		if rule.nullable != nil && *rule.nullable != nullable {
			continue
		}
		score := -1
		if rule.column != "" && columnOverrideMatches(rule.column, col) {
			score = 20
			if rule.nullable != nil {
				score++
			}
		} else if rule.dbType == dbType {
			score = 10
			if rule.nullable != nil {
				score++
			}
		}
		if score > bestScore {
			bestScore, best = score, rule.mapped
		}
	}
	if bestScore >= 0 {
		return best, true
	}
	mt, ok := g.overrides[dbType]
	return mt, ok
}

func columnOverrideMatches(pattern string, col *plugin.Column) bool {
	name := strings.ToLower(col.Name)
	if pattern == name || pattern == "*."+name {
		return true
	}
	if col.Table == nil {
		return false
	}
	table := strings.ToLower(col.Table.Name)
	schema := strings.ToLower(col.Table.Schema)
	catalog := strings.ToLower(col.Table.Catalog)
	return pattern == table+"."+name ||
		(schema != "" && pattern == schema+"."+table+"."+name) ||
		(catalog != "" && pattern == catalog+"."+schema+"."+table+"."+name)
}

func (g *gen) mapType(col *plugin.Column) (mappedType, error) {
	name := dbType(col)
	if col.ArrayDims > 1 {
		return mappedType{}, fmt.Errorf("only one-dimensional PostgreSQL arrays are supported (column %q has %d dimensions)", col.Name, col.ArrayDims)
	}
	if col.IsArray || col.ArrayDims == 1 {
		mt, err := g.mapArray(col)
		if err != nil {
			return mappedType{}, err
		}
		if !col.NotNull {
			mt.OCaml += " option"
			mt.Codec = "Caqti_type.option (" + mt.Codec + ")"
		}
		return mt, nil
	}
	mt, ok := g.matchingOverride(col, name)
	if !ok {
		mt, ok = postgresTypes[name]
	}
	if !ok {
		if enum, exists := g.enumFor(col); exists {
			mt = mappedType{enum.TypeName, enum.TypeName + "_type"}
			ok = true
		}
	}
	if !ok {
		return mappedType{}, fmt.Errorf("unsupported PostgreSQL type %q for column %q; add an override", name, col.Name)
	}
	if !col.NotNull {
		mt.OCaml += " option"
		mt.Codec = "Caqti_type.option (" + mt.Codec + ")"
	}
	return mt, nil
}

func (g *gen) mapArray(col *plugin.Column) (mappedType, error) {
	name, column := dbType(col), col.Name
	var ocaml, encode, decode string
	switch name {
	case "int2", "smallint":
		ocaml, encode, decode = "int", "string_of_int", "(fun s -> try Ok (int_of_string s) with Failure _ -> Error (\"invalid smallint: \" ^ s))"
	case "int4", "integer", "serial":
		ocaml, encode, decode = "int32", "Int32.to_string", "(fun s -> try Ok (Int32.of_string s) with Failure _ -> Error (\"invalid integer: \" ^ s))"
	case "int8", "bigint", "bigserial":
		ocaml, encode, decode = "int64", "Int64.to_string", "(fun s -> try Ok (Int64.of_string s) with Failure _ -> Error (\"invalid bigint: \" ^ s))"
	case "text", "varchar", "character varying", "char", "bpchar", "character":
		ocaml, encode, decode = "string", "(fun x -> x)", "(fun x -> Ok x)"
	case "bool", "boolean":
		ocaml, encode, decode = "bool", "string_of_bool", "(function \"t\" | \"true\" -> Ok true | \"f\" | \"false\" -> Ok false | s -> Error (\"invalid boolean: \" ^ s))"
	case "float4", "real", "float8", "double precision":
		ocaml, encode, decode = "float", "string_of_float", "(fun s -> try Ok (float_of_string s) with Failure _ -> Error (\"invalid float: \" ^ s))"
	case "uuid":
		ocaml, encode, decode = "Uuidm.t", "Uuidm.to_string", "(fun s -> match Uuidm.of_string s with Some x -> Ok x | None -> Error (\"invalid uuid: \" ^ s))"
	default:
		if info, ok := g.enumFor(col); ok {
			enum := info.Enum
			ocaml, encode = info.TypeName, "(fun x -> match x with "
			for _, value := range enum.Vals {
				encode += fmt.Sprintf("| %s -> %q ", constructor(value), value)
			}
			encode += ")"
			decode = "(function "
			for _, value := range enum.Vals {
				decode += fmt.Sprintf("| %q -> Ok %s ", value, constructor(value))
			}
			decode += "| s -> Error (\"invalid enum value: \" ^ s))"
		} else {
			return mappedType{}, fmt.Errorf("PostgreSQL array type %q is not supported for column %q", name, column)
		}
	}
	codec := fmt.Sprintf("Sqlc_runtime.Array.codec ~encode_element:%s ~decode_element:%s", encode, decode)
	return mappedType{ocaml + " list", codec}, nil
}
