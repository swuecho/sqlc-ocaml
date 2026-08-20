// Package plugin contains the JSON representation of sqlc's public codegen
// protocol. It intentionally includes only fields used by this generator.
package plugin

type GenerateRequest struct {
	Settings      *Settings `json:"settings"`
	Catalog       *Catalog  `json:"catalog"`
	Queries       []*Query  `json:"queries"`
	SQLCVersion   string    `json:"sqlc_version"`
	PluginOptions []byte    `json:"plugin_options"`
	GlobalOptions []byte    `json:"global_options"`
}

type GenerateResponse struct {
	Files []*File `json:"files"`
}
type File struct {
	Name     string `json:"name"`
	Contents []byte `json:"contents"`
}
type Settings struct {
	Version string   `json:"version"`
	Engine  string   `json:"engine"`
	Schema  []string `json:"schema"`
	Queries []string `json:"queries"`
}
type Catalog struct {
	Comment       string    `json:"comment"`
	DefaultSchema string    `json:"default_schema"`
	Name          string    `json:"name"`
	Schemas       []*Schema `json:"schemas"`
}
type Schema struct {
	Comment string   `json:"comment"`
	Name    string   `json:"name"`
	Tables  []*Table `json:"tables"`
	Enums   []*Enum  `json:"enums"`
}
type Enum struct {
	Name    string   `json:"name"`
	Vals    []string `json:"vals"`
	Comment string   `json:"comment"`
}
type Table struct {
	Rel     *Identifier `json:"rel"`
	Columns []*Column   `json:"columns"`
	Comment string      `json:"comment"`
}
type Identifier struct {
	Catalog string `json:"catalog"`
	Schema  string `json:"schema"`
	Name    string `json:"name"`
}
type Column struct {
	Name         string      `json:"name"`
	NotNull      bool        `json:"not_null"`
	IsArray      bool        `json:"is_array"`
	Comment      string      `json:"comment"`
	Length       int32       `json:"length"`
	IsNamedParam bool        `json:"is_named_param"`
	IsFuncCall   bool        `json:"is_func_call"`
	Scope        string      `json:"scope"`
	Table        *Identifier `json:"table"`
	TableAlias   string      `json:"table_alias"`
	Type         *Identifier `json:"type"`
	IsSQLCSlice  bool        `json:"is_sqlc_slice"`
	EmbedTable   *Identifier `json:"embed_table"`
	OriginalName string      `json:"original_name"`
	Unsigned     bool        `json:"unsigned"`
	ArrayDims    int32       `json:"array_dims"`
}
type Query struct {
	Text            string       `json:"text"`
	Name            string       `json:"name"`
	Cmd             string       `json:"cmd"`
	Columns         []*Column    `json:"columns"`
	Params          []*Parameter `json:"params"`
	Parameters      []*Parameter `json:"parameters"`
	Comments        []string     `json:"comments"`
	Filename        string       `json:"filename"`
	InsertIntoTable *Identifier  `json:"insert_into_table"`
}
type Parameter struct {
	Number int32   `json:"number"`
	Column *Column `json:"column"`
}

func (q *Query) AllParams() []*Parameter {
	if len(q.Parameters) != 0 {
		return q.Parameters
	}
	return q.Params
}
