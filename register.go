// Package pixellab is a herrscher backend that generates pixel art with PixelLab instead of an LLM.
package pixellab

import (
	"context"

	contracts "github.com/Herrscherd/herrscher-contracts"
)

const backendKind = "pixellab"

func init() {
	texts := newMessages()
	contracts.Register(contracts.Plugin{
		Manifest: contracts.Manifest{
			Kind:     backendKind,
			Category: contracts.CategoryBackend,
			Status:   contracts.StatusLive,
			Config:   settingDeclarations(),
			Capabilities: contracts.Capabilities{
				Gate:    contracts.GrainNone,
				GateWhy: "PixelLab exposes no permission hook",
			},
		},
		Backend: func(_ context.Context, cfg contracts.PluginConfig) (contracts.Backend, error) {
			backend, err := newBackend(cfg, texts)
			if err != nil {
				return nil, err
			}
			return backend, nil
		},
		Probe:   prober{}.probe,
		Catalog: localeRoot{},
	})
}
