package services

import (
	"context"

	"go-conference/core"
)

// IceService exposes the ICE servers browsers should use.
type IceService struct{ p *core.ICEProvider }

func NewIceService(p *core.ICEProvider) *IceService { return &IceService{p: p} }

func (s *IceService) Servers(ctx context.Context) []core.ICEServer { return s.p.ClientServers(ctx) }
