package mcp

// Version is the skillpack server version reported to MCP clients during
// initialization. Bump this when cutting a release.
const Version = "0.1.0"

// Build is stamped at compile time via ldflags in the format
// YYYYMMDD_shorthash (e.g. 20260623_4d64bf7). Defaults to "dev" when not
// set (e.g. go run / go build without ldflags).
var Build = "dev"
