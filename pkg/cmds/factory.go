package cmds

import (
	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/geppetto/pkg/embeddings"
	"github.com/go-go-golems/parka/pkg/handlers"
)

func NewRepositoryFactory() handlers.RepositoryFactory {
	loader := NewElasticSearchCommandLoader(layers.NewSearchClientFromParsedLayers, embeddings.NewSettingsFactoryFromParsedLayers)

	return handlers.NewRepositoryFactoryFromReaderLoaders(loader)
}
