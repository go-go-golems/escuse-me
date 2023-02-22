package pkg

import (
	_ "embed"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
)

//go:embed "flags/es.yaml"
var esFlagsYaml []byte

type EsParameterLayer struct {
	layers.ParameterLayerImpl
}

func NewESParameterLayer() (*EsParameterLayer, error) {
	ret := &EsParameterLayer{}
	err := ret.LoadFromYAML(esFlagsYaml)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
