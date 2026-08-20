//go:build !darwin || !cgo

package collector

func darwinCPUTicks() ([]uint64, []uint64, bool) { return nil, nil, false }
