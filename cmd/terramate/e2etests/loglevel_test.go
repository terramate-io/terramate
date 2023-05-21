package e2etest

import "github.com/rs/zerolog"

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
