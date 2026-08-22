package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/hwu/sqlc-ocaml/internal/generator"
	"github.com/hwu/sqlc-ocaml/internal/plugin"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sqlc-gen-ocaml:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer) error {
	var req plugin.GenerateRequest
	decoder := json.NewDecoder(in)
	if err := decoder.Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode request: unexpected trailing JSON value")
		}
		return fmt.Errorf("decode request trailing data: %w", err)
	}
	resp, err := generator.Generate(&req)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(out).Encode(resp); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}
