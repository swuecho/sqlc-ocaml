package generator

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hwu/sqlc-ocaml/internal/plugin"
)

func ident(name string) *plugin.Identifier { return &plugin.Identifier{Name: name} }
func col(name, typ string, notNull bool) *plugin.Column {
	return &plugin.Column{Name: name, Type: ident(typ), NotNull: notNull}
}

func TestGeneratedOCamlParses(t *testing.T) {
	if _, err := exec.LookPath("ocamlc"); err != nil {
		t.Skip("ocamlc is not installed")
	}
	for _, runtime := range []string{"lwt", "async"} {
		r := request()
		options, _ := json.Marshal(Options{Runtime: runtime})
		r.PluginOptions = options
		resp, err := Generate(r)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		for _, file := range resp.Files {
			path := filepath.Join(dir, file.Name)
			if err := os.WriteFile(path, file.Contents, 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("ocamlc", "-stop-after", "parsing", "-c", path)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s %s does not parse: %v\n%s", runtime, file.Name, err, out)
			}
		}
	}
}

func request() *plugin.GenerateRequest {
	return &plugin.GenerateRequest{
		Settings: &plugin.Settings{Engine: "postgresql"},
		Catalog:  &plugin.Catalog{Schemas: []*plugin.Schema{{Name: "public", Enums: []*plugin.Enum{{Name: "user_status", Vals: []string{"active", "disabled"}}}}}},
		Queries: []*plugin.Query{
			{Name: "FindUser", Cmd: ":one", Text: "SELECT id, email, status FROM users WHERE id = $1", Params: []*plugin.Parameter{{Number: 1, Column: col("id", "bigint", true)}}, Columns: []*plugin.Column{col("id", "bigint", true), col("email", "text", false), col("status", "user_status", true)}},
			{Name: "ListUsers", Cmd: ":many", Text: "SELECT id FROM users", Columns: []*plugin.Column{col("id", "bigint", true)}},
			{Name: "DeleteUser", Cmd: ":exec", Text: "DELETE FROM users WHERE id = $1", Params: []*plugin.Parameter{{Number: 1, Column: col("id", "bigint", true)}}},
		},
	}
}

func TestGenerateMVP(t *testing.T) {
	resp, err := Generate(request())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 2 {
		t.Fatalf("got %d files", len(resp.Files))
	}
	ml := string(resp.Files[0].Contents)
	mli := string(resp.Files[1].Contents)
	for _, want := range []string{"module FindUser = struct", "email : string option;", "status : user_status;", "->!", "->*", "->.", "Db.collect_list", "let fold", "Db.fold request", "{sql|SELECT id, email, status FROM users WHERE id = ?|sql}", "let params_type =", "let row_type =", "let encode_params", "let decode_row", ") sql"} {
		if !strings.Contains(ml, want) {
			t.Errorf("ML missing %q", want)
		}
	}
	if strings.Contains(ml, "$1") || !strings.Contains(ml, "WHERE id = ?") {
		t.Error("PostgreSQL placeholder was not converted to Caqti syntax")
	}
	if got, want := strings.Count(ml, "  let sql =\n"), len(request().Queries); got != want {
		t.Errorf("generated %d SQL bindings, want one for each of %d query modules", got, want)
	}
	if got, want := strings.Count(ml, ") sql"), len(request().Queries); got != want {
		t.Errorf("generated %d requests using extracted SQL, want %d", got, want)
	}
	for _, want := range []string{"(** Query [FindUser] (returns exactly one row). *)", "module FindUser : sig", "(row, [> Caqti_error.call_or_retrieve ]) result Lwt.t", "(unit, [> Caqti_error.call_or_retrieve ]) result Lwt.t"} {
		if !strings.Contains(mli, want) {
			t.Errorf("MLI missing %q", want)
		}
	}
}

func TestGenerateAsyncRuntime(t *testing.T) {
	r := request()
	b, _ := json.Marshal(Options{Filename: "queries", Runtime: "async"})
	r.PluginOptions = b
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	ml, mli := string(resp.Files[0].Contents), string(resp.Files[1].Contents)
	for _, want := range []string{"Caqti_async.CONNECTION", "Async_kernel.Deferred.map", "Db.fold request"} {
		if !strings.Contains(ml, want) {
			t.Errorf("generated Async ML missing %q", want)
		}
	}
	for _, want := range []string{"Caqti_async.CONNECTION", "Async_kernel.Deferred.t", "val fold"} {
		if !strings.Contains(mli, want) {
			t.Errorf("generated Async MLI missing %q", want)
		}
	}
	if strings.Contains(ml, "Caqti_lwt") || strings.Contains(mli, "Lwt.t") {
		t.Fatal("Async output unexpectedly depends on Lwt")
	}
}

func TestRejectsUnknownRuntime(t *testing.T) {
	r := request()
	b, _ := json.Marshal(Options{Runtime: "effects"})
	r.PluginOptions = b
	if _, err := Generate(r); err == nil || !strings.Contains(err.Error(), "invalid runtime") {
		t.Fatalf("unexpected runtime validation error: %v", err)
	}
}

func TestSchemaQualifiedModelsAndEnums(t *testing.T) {
	r := request()
	publicUsers := &plugin.Table{Rel: &plugin.Identifier{Schema: "public", Name: "users"}, Columns: []*plugin.Column{col("id", "bigint", true)}}
	auditUsers := &plugin.Table{Rel: &plugin.Identifier{Schema: "audit", Name: "users"}, Columns: []*plugin.Column{col("id", "bigint", true)}}
	r.Catalog.Schemas = []*plugin.Schema{
		{Name: "public", Tables: []*plugin.Table{publicUsers}, Enums: []*plugin.Enum{{Name: "status", Vals: []string{"active"}}}},
		{Name: "audit", Tables: []*plugin.Table{auditUsers}, Enums: []*plugin.Enum{{Name: "status", Vals: []string{"recorded"}}}},
	}
	r.Queries = []*plugin.Query{{
		Name: "GetUsers", Cmd: ":one", Text: "SELECT 1",
		Columns: []*plugin.Column{
			{Name: "public_user", EmbedTable: &plugin.Identifier{Schema: "public", Name: "users"}},
			{Name: "audit_user", EmbedTable: &plugin.Identifier{Schema: "audit", Name: "users"}},
		},
	}}
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	ml := string(resp.Files[0].Contents)
	for _, want := range []string{"type public_users = {", "type audit_users = {", "type public_status =", "type audit_status =", "public_user : public_users;", "audit_user : audit_users;"} {
		if !strings.Contains(ml, want) {
			t.Errorf("schema-qualified output missing %q\n%s", want, ml)
		}
	}
}

func TestGenerateRepeatedAndReorderedParameters(t *testing.T) {
	r := request()
	r.Queries = []*plugin.Query{{
		Name: "FilterEvents", Cmd: ":many",
		Text: "SELECT id FROM events WHERE owner_id = $2 AND (status = $1 OR status = $1)",
		Params: []*plugin.Parameter{
			{Number: 2, Column: col("owner_id", "bigint", true)},
			{Number: 1, Column: col("status", "text", true)},
		},
		Columns: []*plugin.Column{col("id", "bigint", true)},
	}}
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	ml := string(resp.Files[0].Contents)
	for _, want := range []string{
		"WHERE owner_id = ? AND (status = ? OR status = ?)",
		"Caqti_type.t2 (Caqti_type.t2 (Caqti_type.int64) (Caqti_type.string)) (Caqti_type.string)",
		"((params.owner_id, params.status), params.status)",
	} {
		if !strings.Contains(ml, want) {
			t.Errorf("generated ML missing binding-plan output %q\n%s", want, ml)
		}
	}
}

func TestGenerateNullableParameters(t *testing.T) {
	r := request()
	r.Queries = []*plugin.Query{{
		Name: "PatchUser", Cmd: ":exec",
		Text: "UPDATE users SET email = COALESCE($1, email) WHERE id = $2",
		Params: []*plugin.Parameter{
			{Number: 1, Column: col("email", "text", false)},
			{Number: 2, Column: col("id", "bigint", true)},
		},
	}}
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	ml := string(resp.Files[0].Contents)
	for _, want := range []string{
		"email : string option;",
		"Caqti_type.option (Caqti_type.string)",
		"(params.email, params.id)",
	} {
		if !strings.Contains(ml, want) {
			t.Errorf("generated ML missing nullable parameter output %q\n%s", want, ml)
		}
	}
}

func TestGenerateExecRows(t *testing.T) {
	r := request()
	r.Queries = []*plugin.Query{{
		Name: "DeleteUsers", Cmd: ":execrows", Text: "DELETE FROM users WHERE disabled = $1",
		Params: []*plugin.Parameter{{Number: 1, Column: col("disabled", "bool", true)}},
	}}
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	ml := string(resp.Files[0].Contents)
	mli := string(resp.Files[1].Contents)
	if !strings.Contains(ml, "Db.exec_with_affected_count request (encode_params params)") {
		t.Fatalf("generated ML does not use affected-count execution:\n%s", ml)
	}
	for _, want := range []string{"(int,", "`Unsupported"} {
		if !strings.Contains(mli, want) {
			t.Errorf("generated MLI missing execrows output %q\n%s", want, mli)
		}
	}
}

func TestGenerateEmbeddedModels(t *testing.T) {
	r := request()
	students := &plugin.Table{Rel: ident("students"), Columns: []*plugin.Column{col("id", "bigint", true), col("name", "text", true)}}
	scores := &plugin.Table{Rel: ident("scores"), Columns: []*plugin.Column{col("student_id", "bigint", true), col("score", "integer", true)}}
	r.Catalog.Schemas[0].Tables = []*plugin.Table{students, scores}
	r.Queries = []*plugin.Query{{
		Name: "GetStudentScore", Cmd: ":one", Text: "SELECT students.id, students.name, scores.student_id, scores.score FROM students JOIN scores ON scores.student_id = students.id WHERE students.id = $1",
		Params: []*plugin.Parameter{{Number: 1, Column: col("id", "bigint", true)}},
		Columns: []*plugin.Column{
			{Name: "students", EmbedTable: ident("students")},
			{Name: "scores", EmbedTable: ident("scores")},
		},
	}}
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	ml := string(resp.Files[0].Contents)
	for _, want := range []string{
		"type students = {", "type scores = {",
		"students : students;", "scores : scores;",
		"students = { id = v_students_id; name = v_students_name }",
		"scores = { student_id = v_scores_student_id; score = v_scores_score }",
	} {
		if !strings.Contains(ml, want) {
			t.Errorf("generated ML missing embedded-model output %q\n%s", want, ml)
		}
	}
}

func TestCaqtiSQL(t *testing.T) {
	got, bindings, err := caqtiSQL("SELECT '$1', \"$2\" -- $3\nWHERE a = $2 AND b = $1 OR c = $1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "SELECT '$1', \"$2\" -- $3\nWHERE a = ? AND b = ? OR c = ?" {
		t.Fatalf("unexpected SQL: %s", got)
	}
	if !reflect.DeepEqual(bindings, []int{2, 1, 1}) {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
	if _, _, err := caqtiSQL("SELECT $3", 2); err == nil {
		t.Fatal("expected unknown placeholder error")
	}
}

func TestOverride(t *testing.T) {
	r := request()
	b, _ := json.Marshal(Options{Overrides: []Override{{DBType: "numeric", Type: "Decimal.t", Codec: "Db_types.decimal"}}})
	r.PluginOptions = b
	r.Queries = []*plugin.Query{{Name: "Total", Cmd: ":one", Text: "SELECT total", Columns: []*plugin.Column{col("total", "numeric", true)}}}
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp.Files[0].Contents), "total : Decimal.t;") {
		t.Fatal("override not emitted")
	}
}

func TestColumnAndNullableOverridePrecedence(t *testing.T) {
	r := request()
	email := col("email", "text", false)
	email.Table = ident("users")
	r.Queries = []*plugin.Query{{Name: "Email", Cmd: ":one", Text: "SELECT email FROM users", Columns: []*plugin.Column{email}}}
	nullable := true
	b, _ := json.Marshal(Options{Overrides: []Override{
		{DBType: "pg_catalog.text", Type: "Text.t", Codec: "Text.codec"},
		{Column: "users.email", Nullable: &nullable, Type: "Email.t", Codec: "Email.codec"},
	}})
	r.PluginOptions = b
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	ml := string(resp.Files[0].Contents)
	for _, want := range []string{"email : Email.t option;", "Caqti_type.option (Email.codec)"} {
		if !strings.Contains(ml, want) {
			t.Errorf("generated ML missing specific override %q\n%s", want, ml)
		}
	}
	if strings.Contains(ml, "Text.t") {
		t.Fatal("database-type override won over the more-specific column override")
	}
}

func TestRejectsInvalidOverrideSelector(t *testing.T) {
	r := request()
	b, _ := json.Marshal(Options{Overrides: []Override{{DBType: "text", Column: "users.email", Type: "string", Codec: "Caqti_type.string"}}})
	r.PluginOptions = b
	if _, err := Generate(r); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("unexpected override validation error: %v", err)
	}
}

func TestGenerateIntegerArrayParameter(t *testing.T) {
	r := request()
	ids := col("ids", "bigint", true)
	ids.IsArray = true
	r.Queries = []*plugin.Query{{
		Name: "FindUsers", Cmd: ":many", Text: "SELECT id FROM users WHERE id = ANY($1::bigint[])",
		Params: []*plugin.Parameter{{Number: 1, Column: ids}}, Columns: []*plugin.Column{col("id", "bigint", true)},
	}}
	resp, err := Generate(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 4 || resp.Files[2].Name != "sqlc_runtime.ml" || resp.Files[3].Name != "sqlc_runtime.mli" {
		t.Fatalf("unexpected array runtime files: %#v", resp.Files)
	}
	ml := string(resp.Files[0].Contents)
	for _, want := range []string{"ids : int64 list;", "Sqlc_runtime.Array.codec", "~encode_element:Int64.to_string", "ANY(?::bigint[])", "params.ids"} {
		if !strings.Contains(ml, want) {
			t.Errorf("generated ML missing integer-array output %q\n%s", want, ml)
		}
	}
}

func TestRejectsUnsupportedCommandAndArray(t *testing.T) {
	r := request()
	r.Queries[0].Cmd = ":copyfrom"
	if _, err := Generate(r); err == nil {
		t.Fatal("expected command error")
	}
	r = request()
	r.Queries[0].Params[0].Column.IsArray = true
	r.Queries[0].Params[0].Column.Type = ident("text")
	r.Queries[0].Params[0].Column.ArrayDims = 2
	if _, err := Generate(r); err == nil {
		t.Fatal("expected multidimensional-array error")
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{"FindHTTPUser": "find_h_t_t_p_user", "type": "type_", "123-name": "v_123_name"} {
		if got := snake(in); got != want {
			t.Errorf("snake(%q)=%q want %q", in, got, want)
		}
	}
}

func TestQualifiedPostgresType(t *testing.T) {
	g := &gen{overrides: map[string]mappedType{}, enums: map[string]enumInfo{}}
	mapped, err := g.mapType(col("completed", "pg_catalog.bool", true))
	if err != nil {
		t.Fatal(err)
	}
	if mapped.OCaml != "bool" {
		t.Fatalf("got %q", mapped.OCaml)
	}
}

func TestNormalizedIR(t *testing.T) {
	r := request()
	r.Queries[0].Filename = "users.sql"
	g := &gen{req: r, overrides: map[string]mappedType{}, enums: map[string]enumInfo{
		"public.user_status": {Enum: r.Catalog.Schemas[0].Enums[0], Schema: "public", TypeName: "user_status"},
		"user_status":        {Enum: r.Catalog.Schemas[0].Enums[0], Schema: "public", TypeName: "user_status"},
	}, enumList: []enumInfo{{Enum: r.Catalog.Schemas[0].Enums[0], Schema: "public", TypeName: "user_status"}}}
	program, err := g.normalize()
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Enums) != 1 || program.Enums[0].TypeName != "user_status" {
		t.Fatalf("unexpected enums: %#v", program.Enums)
	}
	if len(program.Queries) != 3 {
		t.Fatalf("got %d queries", len(program.Queries))
	}
	find := program.Queries[0]
	if find.SourceName != "FindUser" || find.SourceFile != "users.sql" || find.ModuleName != "FindUser" || find.Cardinality != One {
		t.Fatalf("unexpected query identity: %#v", find)
	}
	if find.SQL != "SELECT id, email, status FROM users WHERE id = ?" {
		t.Fatalf("unexpected normalized SQL: %s", find.SQL)
	}
	if find.Params.TypeName != "params" || len(find.Params.Fields) != 1 || find.Params.Fields[0].Type.Name != "int64" {
		t.Fatalf("unexpected params: %#v", find.Params)
	}
	if !reflect.DeepEqual(find.Bindings, []ParameterBinding{{FieldIndex: 0}}) {
		t.Fatalf("unexpected parameter bindings: %#v", find.Bindings)
	}
	if find.Row == nil || len(find.Row.Fields) != 3 {
		t.Fatalf("unexpected row: %#v", find.Row)
	}
	email := find.Row.Fields[1]
	if email.Name != "email" || email.Type.DatabaseType != "text" || email.Type.Name != "string option" || !email.Type.Nullable {
		t.Fatalf("unexpected nullable field: %#v", email)
	}
	if program.Queries[1].Cardinality != Many || program.Queries[2].Cardinality != Exec || program.Queries[2].Row != nil {
		t.Fatal("cardinality was not normalized")
	}
}

func TestNormalizeRejectsMissingParameterMetadata(t *testing.T) {
	r := request()
	r.Queries[0].Params[0].Column = nil
	g := &gen{req: r, overrides: map[string]mappedType{}, enums: map[string]enumInfo{}}
	if _, err := g.normalize(); err == nil || !strings.Contains(err.Error(), "has no column metadata") {
		t.Fatalf("unexpected error: %v", err)
	}
}
