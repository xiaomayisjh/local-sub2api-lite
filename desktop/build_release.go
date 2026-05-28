//go:build !debug

package main

// IsDebugBuild reports whether this binary was built with the "debug" build tag.
// When false, DevTools and the system console are off unless overridden via env vars.
const IsDebugBuild = false
