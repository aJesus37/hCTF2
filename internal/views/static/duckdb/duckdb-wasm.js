// This is a shim that exports the DuckDB WASM module
// The actual files are served via HTTP from /static/duckdb/

// Re-export everything from the CDN version
export * from 'https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@latest/+esm';

console.log('DuckDB WASM loaded from local fallback');
