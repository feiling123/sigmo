package network

import (
	"context"
	"errors"
	"fmt"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
	"github.com/damonto/sigmo/internal/pkg/storage"
)

type network struct {
	preferences *mmodem.NetworkPreferences
	store       *storage.Store
}

var errNetworkPreferencesRequired = errors.New("network preferences are required")

func newNetwork(preferences *mmodem.NetworkPreferences, store *storage.Store) (*network, error) {
	if preferences == nil {
		return nil, errNetworkPreferencesRequired
	}
	if store == nil {
		return nil, errNetworkRegistrationStorageRequired
	}
	return &network{preferences: preferences, store: store}, nil
}

func (n *network) List(ctx context.Context, modem *mmodem.Modem) ([]NetworkResponse, error) {
	networks, err := modem.ThreeGPP().ScanNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan networks: %w", err)
	}

	response := make([]NetworkResponse, 0, len(networks))
	for _, network := range networks {
		response = append(response, NetworkResponse{
			Status:             network.Status.String(),
			OperatorName:       network.OperatorName,
			OperatorShortName:  network.OperatorShortName,
			OperatorCode:       network.OperatorCode,
			AccessTechnologies: accessTechnologyStrings(network.AccessTechnology),
		})
	}
	return response, nil
}

func accessTechnologyStrings(access []mmodem.ModemAccessTechnology) []string {
	names := make([]string, 0, len(access))
	for _, tech := range access {
		names = append(names, tech.String())
	}
	return names
}
