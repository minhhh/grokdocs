package project

import (
	"os"

	"github.com/minhhh/grokdocs/internal/util"
)

// AssertCollectionValid exits the process if name is not a configured collection.
func AssertCollectionValid(p *Project, name string) {
	if _, ok := p.Config.Collections[name]; !ok {
		util.Logger.Error().Str("collection", name).Msg("unknown collection")
		os.Exit(1)
	}
}
