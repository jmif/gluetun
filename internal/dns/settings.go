package dns

import (
	"context"
	"fmt"

	"github.com/qdm12/dns/v2/pkg/dot"
	cachemiddleware "github.com/qdm12/dns/v2/pkg/middlewares/cache"
	"github.com/qdm12/dns/v2/pkg/middlewares/cache/lru"
	filtermiddleware "github.com/qdm12/dns/v2/pkg/middlewares/filter"
	"github.com/qdm12/dns/v2/pkg/middlewares/filter/mapfilter"
	"github.com/qdm12/dns/v2/pkg/provider"
	"github.com/qdm12/dns/v2/pkg/server"
	"github.com/qdm12/gluetun/internal/configuration/settings"
	splitmiddleware "github.com/qdm12/gluetun/internal/dns/middleware/split"
)

func (l *Loop) GetSettings() (settings settings.DNS) { return l.state.GetSettings() }

func (l *Loop) SetSettings(ctx context.Context, settings settings.DNS) (
	outcome string,
) {
	return l.state.SetSettings(ctx, settings)
}

func buildDoTSettings(settings settings.DNS,
	filter *mapfilter.Filter, logger Logger, bypassConfig *BypassConfig) (
	serverSettings server.Settings, err error,
) {
	serverSettings.Logger = logger

	var dotSettings dot.Settings
	providersData := provider.NewProviders()
	dotSettings.UpstreamResolvers = make([]provider.Provider, len(settings.DoT.Providers))
	for i := range settings.DoT.Providers {
		var err error
		dotSettings.UpstreamResolvers[i], err = providersData.Get(settings.DoT.Providers[i])
		if err != nil {
			panic(err) // this should already had been checked
		}
	}
	dotSettings.IPVersion = "ipv4"
	if *settings.DoT.IPv6 {
		dotSettings.IPVersion = "ipv6"
	}

	serverSettings.Dialer, err = dot.New(dotSettings)
	if err != nil {
		return server.Settings{}, fmt.Errorf("creating DNS over TLS dialer: %w", err)
	}

	// Add DNS bypass middleware if configured
	if bypassConfig != nil && bypassConfig.Resolver.IsValid() && len(bypassConfig.Domains) > 0 {
		splitMiddleware, err := splitmiddleware.New(splitmiddleware.Settings{
			BypassResolver: bypassConfig.Resolver,
			BypassDomains:  bypassConfig.Domains,
			Logger:         logger,
		})
		if err != nil {
			return server.Settings{}, fmt.Errorf("creating DNS bypass middleware: %w", err)
		}
		serverSettings.Middlewares = append(serverSettings.Middlewares, splitMiddleware)
		logger.Info(fmt.Sprintf("DNS bypass enabled for domains: %v using resolver: %s",
			bypassConfig.Domains, bypassConfig.Resolver))
	}

	if *settings.DoT.Caching {
		lruCache, err := lru.New(lru.Settings{})
		if err != nil {
			return server.Settings{}, fmt.Errorf("creating LRU cache: %w", err)
		}
		cacheMiddleware, err := cachemiddleware.New(cachemiddleware.Settings{
			Cache: lruCache,
		})
		if err != nil {
			return server.Settings{}, fmt.Errorf("creating cache middleware: %w", err)
		}
		serverSettings.Middlewares = append(serverSettings.Middlewares, cacheMiddleware)
	}

	filterMiddleware, err := filtermiddleware.New(filtermiddleware.Settings{
		Filter: filter,
	})
	if err != nil {
		return server.Settings{}, fmt.Errorf("creating filter middleware: %w", err)
	}
	serverSettings.Middlewares = append(serverSettings.Middlewares, filterMiddleware)

	return serverSettings, nil
}
