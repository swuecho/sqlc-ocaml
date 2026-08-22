package generator

import "testing"

func FuzzCaqtiSQL(f *testing.F) {
	for _, sql := range []string{
		"SELECT $1",
		"SELECT '$1', $1",
		"SELECT $$ $1 $$, $1",
		"SELECT /* $1 */ $1 -- $1\n",
		"SELECT E'can\\'t $1', $1",
	} {
		f.Add(sql, uint8(1))
	}
	f.Fuzz(func(t *testing.T, sql string, rawParams uint8) {
		params := int(rawParams % 32)
		got, occurrences, err := caqtiSQL(sql, params)
		if err != nil {
			return
		}
		for _, number := range occurrences {
			if number < 1 || number > params {
				t.Fatalf("out-of-range occurrence %d for %d parameters", number, params)
			}
		}
		if len(got) > len(sql) {
			t.Fatalf("placeholder rewriting unexpectedly grew SQL from %d to %d bytes", len(sql), len(got))
		}
	})
}
