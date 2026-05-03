// Package snapshot provides helpers for packaging and extracting workspace
// snapshots as tar.gz archives.
//
// It is intended for short-lived filesystem snapshots used by the local
// pipeline execution flow: package a workspace directory into an archive, then
// unpack that archive into an isolated working directory on the execution side.
package snapshot
