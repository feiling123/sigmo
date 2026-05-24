//go:build esim_transfer

package esim

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	sgp22 "github.com/damonto/euicc-go/v2"
	"github.com/damonto/ts43-go/sim"
	"github.com/damonto/ts43-go/ts43"
	"github.com/labstack/echo/v5"

	"github.com/damonto/sigmo/internal/app/httpapi"
	"github.com/damonto/sigmo/internal/pkg/carrier"
	"github.com/damonto/sigmo/internal/pkg/config"
	ilpa "github.com/damonto/sigmo/internal/pkg/lpa"
	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

func transferProfileError(c *echo.Context, err error) error {
	if errors.Is(err, errSourceIMEIRequired) {
		return httpapi.BadRequest(c, errorCodeTransferSourceIMEIRequired, err)
	}
	if errors.Is(err, mmodem.ErrNotFound) {
		return httpapi.NotFound(c, errorCodeTransferSourceNotFound, err)
	}
	if errors.Is(err, errTransferSourceUnsupported) {
		return httpapi.BadRequest(c, errorCodeTransferSourceUnsupported, err)
	}
	return httpapi.Internal(c, errorCodeListTransferProfilesFailed, err)
}

func (h *Handler) transferProfiles(ctx context.Context, cfg *config.Config, req TransferProfilesRequest) ([]TransferProfileResponse, error) {
	candidates, err := h.transferProfileCandidates(ctx, cfg, req)
	if err != nil {
		return nil, err
	}
	response := make([]TransferProfileResponse, 0, len(candidates))
	for _, candidate := range candidates {
		response = append(response, candidate.response)
	}
	return response, nil
}

func (h *Handler) transferProfileCandidates(ctx context.Context, cfg *config.Config, req TransferProfilesRequest) ([]transferProfileCandidate, error) {
	switch req.SourceType {
	case transferSourceModem:
		return h.modemTransferProfileCandidates(ctx, cfg, req)
	case transferSourceCCID:
		return h.ccidTransferProfileCandidates(ctx, cfg, req)
	default:
		return nil, errTransferSourceUnsupported
	}
}

func (h *Handler) modemTransferProfileCandidates(ctx context.Context, cfg *config.Config, req TransferProfilesRequest) ([]transferProfileCandidate, error) {
	modem, err := h.registry.Find(ctx, req.SourceID)
	if err != nil {
		return nil, err
	}
	profiles, err := sourceModemProfiles(modem, cfg)
	if err == nil {
		return esimTransferCandidates(profiles), nil
	}
	if !errors.Is(err, ilpa.ErrNoSupportedAID) {
		return nil, err
	}
	return h.physicalTransferProfileCandidates(ctx, cfg, req)
}

func (h *Handler) ccidTransferProfileCandidates(ctx context.Context, cfg *config.Config, req TransferProfilesRequest) ([]transferProfileCandidate, error) {
	profiles, err := sourceCCIDProfiles(cfg, req.SourceID)
	if err == nil {
		return esimTransferCandidates(profiles), nil
	}
	if !errors.Is(err, ilpa.ErrNoSupportedAID) {
		return nil, err
	}
	return h.physicalTransferProfileCandidates(ctx, cfg, req)
}

func (h *Handler) physicalTransferProfileCandidates(ctx context.Context, cfg *config.Config, req TransferProfilesRequest) ([]transferProfileCandidate, error) {
	source, err := h.openSource(ctx, cfg, transferStart{
		SourceType: req.SourceType,
		SourceID:   req.SourceID,
		SourceIMEI: req.SourceIMEI,
	})
	if err != nil {
		return nil, err
	}
	defer source.Close()

	identity, err := source.channel.Identity(ctx)
	if err != nil {
		return nil, fmt.Errorf("read source SIM identity: %w", err)
	}
	candidate := physicalTransferCandidate(*identity)
	return []transferProfileCandidate{candidate}, nil
}

func sourceCCIDProfiles(cfg *config.Config, sourceID string) ([]*sgp22.ProfileInfo, error) {
	reader, err := openCCIDLPAReader(sourceID)
	if err != nil {
		return nil, fmt.Errorf("open CCID reader: %w", err)
	}
	client, err := ilpa.NewWithChannel(sourceLockKey(transferSourceCCID, sourceID), "", reader, cfg)
	if err != nil {
		return nil, fmt.Errorf("create source LPA client: %w", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			slog.Warn("close source LPA client", "error", cerr)
		}
	}()
	profiles, err := client.ListProfile(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("list source profiles: %w", err)
	}
	return profiles, nil
}

func esimTransferCandidates(profiles []*sgp22.ProfileInfo) []transferProfileCandidate {
	response := make([]transferProfileCandidate, 0, len(profiles))
	for _, profile := range profiles {
		response = append(response, esimTransferCandidate(profile))
	}
	return response
}

func esimTransferCandidate(profile *sgp22.ProfileInfo) transferProfileCandidate {
	mcc := profile.ProfileOwner.MCC()
	mnc := profile.ProfileOwner.MNC()
	gid1 := strings.ToUpper(hex.EncodeToString(profile.ProfileOwner.GID1))
	enabled := profile.ProfileState == sgp22.ProfileEnabled
	supported, reason := transferSupport(sim.Identity{MCC: mcc, MNC: mnc, GID1: gid1}, ts43.SIMTypeESIM, "eSIM")
	carrierName := transferCarrierName(mcc + mnc)
	return transferProfileCandidate{
		response: TransferProfileResponse{
			ID:                  profile.ICCID.String(),
			Type:                transferProfileESIM,
			Name:                profileDisplayName(profile),
			ServiceProviderName: profile.ServiceProviderName,
			ICCID:               profile.ICCID.String(),
			Icon:                profileIconDataURL(profile.Icon),
			RegionCode:          profileRegion(profile),
			Enabled:             enabled,
			Supported:           supported,
			UnsupportedReason:   reason,
			CarrierName:         carrierName,
		},
	}
}

func physicalTransferCandidate(identity sim.Identity) transferProfileCandidate {
	carrierInfo := carrier.Lookup(identity.MCCMNC())
	supported, reason := transferSupport(identity, ts43.SIMTypePSIM, "pSIM")
	name := carrierInfo.Name
	if name == "" || name == "Unknown" {
		name = identity.MCCMNC()
	}
	carrierName := name
	if carrierInfo.Name == "Unknown" {
		carrierName = ""
	}
	return transferProfileCandidate{
		response: TransferProfileResponse{
			ID:                "physical:" + identity.ICCID,
			Type:              transferProfilePhysical,
			Name:              name,
			ICCID:             identity.ICCID,
			RegionCode:        carrierInfo.Region,
			Enabled:           true,
			Supported:         supported,
			UnsupportedReason: reason,
			CarrierName:       carrierName,
		},
	}
}

func transferSupport(identity sim.Identity, sourceSIMType ts43.SIMType, label string) (bool, string) {
	if _, err := ts43.DiscoverEntitlement(identity, sourceSIMType); err != nil {
		if errors.Is(err, ts43.ErrEntitlementAmbiguous) {
			return false, "carrier entitlement config is ambiguous"
		}
		if errors.Is(err, ts43.ErrEntitlementUnsupported) || errors.Is(err, ts43.ErrEntitlementNotFound) {
			return false, "carrier does not support " + label + " transfer"
		}
		return false, "carrier does not support " + label + " transfer"
	}
	return true, ""
}

func transferCarrierName(mccmnc string) string {
	carrierInfo := carrier.Lookup(mccmnc)
	if carrierInfo.Name == "Unknown" {
		return ""
	}
	return carrierInfo.Name
}

func findTransferCandidate(profiles []transferProfileCandidate, id string) (transferProfileCandidate, bool) {
	for _, profile := range profiles {
		if profile.response.ID != id {
			continue
		}
		return profile, true
	}
	return transferProfileCandidate{}, false
}
