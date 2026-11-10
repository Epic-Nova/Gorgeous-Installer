//go:build !windows

package settings

func UpdateRegistryAssociation(*AppSettings) error { return nil }
