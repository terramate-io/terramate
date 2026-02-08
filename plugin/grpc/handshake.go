// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"os"

	"github.com/hashicorp/go-plugin"
)

const (
	handshakeProtocolVersion = 1
	magicCookieKey           = "TM_PLUGIN_MAGIC_COOKIE"
	magicCookieValue         = "terramate"
)

// HandshakeConfig returns the shared handshake configuration for Terramate plugins.
func HandshakeConfig() plugin.HandshakeConfig {
	return plugin.HandshakeConfig{
		ProtocolVersion:  handshakeProtocolVersion,
		MagicCookieKey:   magicCookieKey,
		MagicCookieValue: magicCookieValue,
	}
}

// IsPluginEnv reports whether the current process is running as a plugin.
func IsPluginEnv() bool {
	return os.Getenv(magicCookieKey) == magicCookieValue
}
