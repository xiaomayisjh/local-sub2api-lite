//go:build debug

package main

// IsDebugBuild is true for binaries built with `-tags debug`.
// Triggers DevTools open-on-startup, context menu, attached console window on Windows,
// and any other developer-only conveniences — all without needing env vars at runtime.
const IsDebugBuild = true
