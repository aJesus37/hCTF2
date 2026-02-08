package main

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed internal/views/static
var staticFS embed.FS

func main() {
	fmt.Println("Files in staticFS:")
	fs.WalkDir(staticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, _ := d.Info()
			fmt.Printf("  %s (%d bytes)\n", path, info.Size())
		}
		return nil
	})

	fmt.Println("\nTrying to open duckdb-mvp.wasm...")
	f, err := staticFS.Open("internal/views/static/duckdb/duckdb-mvp.wasm")
	if err != nil {
		fmt.Printf("❌ Error with full path: %v\n", err)
		f, err = staticFS.Open("duckdb/duckdb-mvp.wasm")
		if err != nil {
			fmt.Printf("❌ Error with short path: %v\n", err)
		} else {
			fmt.Println("✅ Found with duckdb/duckdb-mvp.wasm")
		}
	} else {
		fmt.Println("✅ Found with full path")
		f.Close()
	}
}
