//go:build esim_transfer

package esim

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/damonto/ts43-go/ts43"
	"github.com/labstack/echo/v5"

	"github.com/damonto/sigmo/internal/app/httpapi"
	"github.com/damonto/sigmo/internal/pkg/config"
	ilpa "github.com/damonto/sigmo/internal/pkg/lpa"
	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

type transferSourceType string
type transferProfileType string

const (
	transferSourceModem transferSourceType = "modem"
	transferSourceCCID  transferSourceType = "ccid"

	transferProfileESIM     transferProfileType = "esim"
	transferProfilePhysical transferProfileType = "physical"

	errorCodeListTransferSourcesFailed  = "list_transfer_sources_failed"
	errorCodeListTransferProfilesFailed = "list_transfer_profiles_failed"
	errorCodeTransferInvalidRequest     = "transfer_invalid_request"
	errorCodeTransferSourceIMEIRequired = "transfer_source_imei_required"
	errorCodeTransferSourceNotFound     = "transfer_source_not_found"
	errorCodeTransferSourceUnsupported  = "transfer_source_unsupported"
	errorCodeTransferProfileUnsupported = "transfer_profile_unsupported"
	errorCodeTransferESIMFailed         = "transfer_esim_failed"
)

const (
	transferStagePreparing   = "preparing"
	transferStageCarrier     = "carrier"
	transferStageDownloading = "downloading"
	transferStageEnabling    = "enabling"
	transferStageCompleting  = "completing"
	transferStageDeleting    = "deleting"
)

const (
	ts43DeviceVendor          = "Google"
	ts43DeviceModel           = "Pixel 8 Pro"
	ts43DeviceSoftwareVersion = "15/AP3A.240905.015"
)

const (
	scardNoServiceName = "SCARD_E_NO_SERVICE"
	scardNoServiceCode = "0x8010001D"
)

var (
	errSourceIMEIRequired         = errors.New("source IMEI is required")
	errTransferSourceUnsupported  = errors.New("transfer source is unsupported")
	errTransferProfileUnsupported = errors.New("transfer profile is unsupported")
	errWebsheetUnsupported        = errors.New("carrier websheet is unsupported")
	errCarrierDismissed           = errors.New("carrier dismissed transfer")
	errSourceDeletionDeclined     = errors.New("carrier requires source profile deletion")
	errPhysicalSourceDeletion     = errors.New("carrier requested physical SIM deletion")
	errTransferSourceIsTarget     = errors.New("transfer source cannot be target modem")
)

type transferStart struct {
	SourceType transferSourceType
	SourceID   string
	ProfileID  string
	SourceIMEI string
}

type transferProfileCandidate struct {
	response TransferProfileResponse
}

type sourceEndpoint struct {
	channel       ts43.Channel
	release       func()
	device        ts43.Device
	sourceSIMType ts43.SIMType
}

func (s *sourceEndpoint) Close() {
	if s == nil || s.release == nil {
		return
	}
	s.release()
	s.release = nil
}

type activeTransfer struct {
	cfg          *config.Config
	source       *sourceEndpoint
	target       *mmodem.Modem
	targetClient *ilpa.LPA
	client       *ts43.Client
	targetClosed bool
}

func (t *activeTransfer) Close() {
	if t == nil {
		return
	}
	t.CloseTarget()
	t.source.Close()
}

func (t *activeTransfer) CloseTarget() {
	if t == nil || t.targetClosed || t.targetClient == nil {
		return
	}
	t.targetClosed = true
	if cerr := t.targetClient.Close(); cerr != nil {
		slog.Warn("close target LPA client", "error", cerr)
	}
}

func (h *Handler) TransferSources(c *echo.Context) error {
	ctx := c.Request().Context()
	target, err := h.registry.Find(ctx, c.Param("id"))
	if err != nil {
		return httpapi.ModemLookupError(c, err, errorCodeListTransferSourcesFailed)
	}
	modems, err := h.registry.Modems(ctx)
	if err != nil {
		return httpapi.Internal(c, errorCodeListTransferSourcesFailed, fmt.Errorf("list modems: %w", err))
	}
	cfg := h.configSnapshot()
	response := make([]TransferSourceResponse, 0, len(modems))
	for _, modem := range modems {
		if modem.EquipmentIdentifier == "" || modem.EquipmentIdentifier == target.EquipmentIdentifier {
			continue
		}
		response = append(response, TransferSourceResponse{
			Type:   transferSourceModem,
			ID:     modem.EquipmentIdentifier,
			Name:   transferModemName(cfg, modem),
			Detail: modem.EquipmentIdentifier,
		})
	}
	readers, ccidErr := listCCIDReaders()
	for _, reader := range readers {
		response = append(response, TransferSourceResponse{
			Type:               transferSourceCCID,
			ID:                 reader,
			Name:               reader,
			RequiresSourceIMEI: true,
		})
	}
	out := TransferSourcesResponse{Sources: response}
	if ccidErr != nil {
		out.CCIDError = ccidErr.Error()
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) TransferProfiles(c *echo.Context) error {
	ctx := c.Request().Context()
	target, err := h.registry.Find(ctx, c.Param("id"))
	if err != nil {
		return httpapi.ModemLookupError(c, err, errorCodeListTransferProfilesFailed)
	}
	var req TransferProfilesRequest
	if err := c.Bind(&req); err != nil {
		return httpapi.BadRequest(c, errorCodeTransferInvalidRequest, err)
	}
	req = normalizeTransferProfilesRequest(req)
	if err := validateTransferTarget(target, transferStart{SourceType: req.SourceType, SourceID: req.SourceID}); err != nil {
		return httpapi.BadRequest(c, errorCodeTransferInvalidRequest, err)
	}
	cfg := h.configSnapshot()
	profiles, err := h.transferProfiles(ctx, cfg, req)
	if err != nil {
		return transferProfileError(c, err)
	}
	return c.JSON(http.StatusOK, profiles)
}

func (h *Handler) Transfer(c *echo.Context) error {
	ctx := c.Request().Context()
	target, err := h.registry.Find(ctx, c.Param("id"))
	if err != nil {
		return httpapi.ModemLookupError(c, err, errorCodeTransferESIMFailed)
	}
	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	transferCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	session := newTransferSession(conn, cancel)
	startMsg, ok := session.waitForStart(transferCtx)
	if !ok {
		return nil
	}
	start := transferStart{
		SourceType: transferSourceType(strings.TrimSpace(string(startMsg.SourceType))),
		SourceID:   strings.TrimSpace(startMsg.SourceID),
		ProfileID:  strings.TrimSpace(startMsg.ProfileID),
		SourceIMEI: strings.TrimSpace(startMsg.SourceIMEI),
	}
	if err := h.runTransfer(transferCtx, session, target, start); err != nil {
		_ = session.send(transferServerMessage{Type: wsTypeTransferError, Message: err.Error()})
		return nil
	}
	_ = session.send(transferServerMessage{Type: wsTypeTransferCompleted})
	return nil
}

func (h *Handler) configSnapshot() *config.Config {
	cfg := h.provisioning.store.Snapshot()
	return &cfg
}

func (h *Handler) runTransfer(ctx context.Context, session *transferSession, target *mmodem.Modem, start transferStart) error {
	if err := validateTransferStart(start); err != nil {
		return err
	}
	if err := validateTransferTarget(target, start); err != nil {
		return err
	}
	session.sendIfConnected(transferServerMessage{Type: wsTypeProgress, Stage: transferStagePreparing})

	transfer, err := h.prepareTransfer(ctx, target, start)
	if err != nil {
		return err
	}
	defer transfer.Close()

	session.sendIfConnected(transferServerMessage{Type: wsTypeProgress, Stage: transferStageCarrier})
	result, err := transfer.client.Transfer(ctx)
	if err != nil {
		return fmt.Errorf("start transfer: %w", err)
	}
	for {
		var done bool
		result, done, err = h.handleTransferEvent(ctx, session, transfer, start, result)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

func validateTransferStart(start transferStart) error {
	if start.SourceType == "" || start.SourceID == "" || start.ProfileID == "" {
		return errors.New("source and profile are required")
	}
	switch start.SourceType {
	case transferSourceModem:
	case transferSourceCCID:
		if start.SourceIMEI == "" {
			return errSourceIMEIRequired
		}
	default:
		return errTransferSourceUnsupported
	}
	return nil
}

func normalizeTransferProfilesRequest(req TransferProfilesRequest) TransferProfilesRequest {
	return TransferProfilesRequest{
		SourceType: transferSourceType(strings.TrimSpace(string(req.SourceType))),
		SourceID:   strings.TrimSpace(req.SourceID),
		SourceIMEI: strings.TrimSpace(req.SourceIMEI),
	}
}

func validateTransferTarget(target *mmodem.Modem, start transferStart) error {
	if target == nil || start.SourceType != transferSourceModem {
		return nil
	}
	if start.SourceID == target.EquipmentIdentifier {
		return errTransferSourceIsTarget
	}
	return nil
}

func (h *Handler) prepareTransfer(ctx context.Context, target *mmodem.Modem, start transferStart) (*activeTransfer, error) {
	cfg := h.configSnapshot()
	candidates, err := h.transferProfileCandidates(ctx, cfg, TransferProfilesRequest{
		SourceType: start.SourceType,
		SourceID:   start.SourceID,
		SourceIMEI: start.SourceIMEI,
	})
	if err != nil {
		return nil, err
	}
	candidate, ok := findTransferCandidate(candidates, start.ProfileID)
	if !ok || !candidate.response.Supported {
		return nil, errTransferProfileUnsupported
	}
	if err := h.activateSourceProfile(ctx, cfg, start, candidate); err != nil {
		return nil, err
	}

	source, err := h.openSource(ctx, cfg, start)
	if err != nil {
		return nil, err
	}
	source.sourceSIMType = ts43SourceSIMType(candidate.response.Type)
	source.device.ICCID = candidate.response.ICCID
	releaseSource := true
	defer func() {
		if releaseSource {
			source.Close()
		}
	}()

	targetClient, err := ilpa.New(target, cfg)
	if err != nil {
		return nil, fmt.Errorf("create target LPA client: %w", err)
	}
	releaseTarget := true
	defer func() {
		if releaseTarget {
			if cerr := targetClient.Close(); cerr != nil {
				slog.Warn("close target LPA client", "error", cerr)
			}
		}
	}()

	targetIMEI, err := target.ThreeGPP().IMEI(ctx)
	if err != nil {
		return nil, fmt.Errorf("read target IMEI: %w", err)
	}
	eid, err := targetClient.EID()
	if err != nil {
		return nil, fmt.Errorf("read target EID: %w", err)
	}
	targetDevice := ts43TransferDevice(targetIMEI)
	targetDevice.EID = strings.ToUpper(hex.EncodeToString(eid))
	client, err := ts43.New(&ts43.Config{
		Logger:      slog.Default(),
		Entitlement: ts43.Entitlement{SourceSIMType: source.sourceSIMType},
		Source:      ts43.Endpoint{Channel: source.channel, Device: source.device},
		Target: ts43.Endpoint{
			Device: targetDevice,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create TS.43 client: %w", err)
	}

	releaseSource = false
	releaseTarget = false
	return &activeTransfer{
		cfg:          cfg,
		source:       source,
		target:       target,
		targetClient: targetClient,
		client:       client,
	}, nil
}

func (h *Handler) handleTransferEvent(ctx context.Context, session *transferSession, transfer *activeTransfer, start transferStart, result *ts43.Result) (*ts43.Result, bool, error) {
	switch event := result.Event.(type) {
	case ts43.UserInputEvent:
		slog.Info("TS.43 transfer requires user input")
		answer, err := session.userInput(ctx, event)
		if err != nil {
			slog.Warn("TS.43 user input interrupted", "error", err)
			return result, false, err
		}
		next, err := transfer.client.Continue(ctx, result, ts43.ContinueRequest{UserInput: answer})
		if err != nil {
			slog.Warn("continue TS.43 transfer after user input", "error", err)
		}
		return next, false, err
	case ts43.WebsheetEvent:
		slog.Warn("TS.43 websheet event is unsupported", "contentType", event.Websheet.ContentsType)
		return result, false, errWebsheetUnsupported
	case ts43.SourceProfileDeletionEvent:
		slog.Info("TS.43 transfer requires source profile deletion", "iccid", event.ICCID)
		next, err := h.handleSourceProfileDeletion(ctx, session, transfer, start, result, event)
		if err != nil {
			slog.Warn("handle TS.43 source profile deletion", "iccid", event.ICCID, "error", err)
		}
		return next, false, err
	case ts43.DownloadReadyEvent:
		slog.Info("TS.43 transfer download is ready", "smdp", event.Config.SMDPFQDN, "profileICCID", event.Config.ProfileICCID)
		next, err := h.downloadEnableAndComplete(ctx, session, transfer, result, event.Config)
		if err != nil {
			slog.Warn("download TS.43 transferred profile", "error", err)
		}
		return next, false, err
	case ts43.SMDSDiscoveryEvent:
		next, err := h.handleSMDSDiscovery(ctx, session, transfer, result, event)
		return next, false, err
	case ts43.DelayedDownloadEvent:
		next, err := h.handleSMDSDiscovery(ctx, session, transfer, result, smdsDiscoveryEventFromDelayedDownload(event))
		return next, false, err
	case ts43.ActivationPendingEvent:
		slog.Info("TS.43 transfer activation is pending", "iccid", event.ICCID, "subscriptionResult", event.SubscriptionResult)
		return result, true, nil
	case ts43.DoneEvent:
		slog.Info("TS.43 transfer completed")
		return result, true, nil
	case ts43.DismissEvent:
		slog.Warn("TS.43 transfer dismissed by carrier", "subscriptionResult", event.SubscriptionResult)
		return result, false, errCarrierDismissed
	default:
		slog.Warn("unexpected TS.43 transfer event", "event", fmt.Sprintf("%T", result.Event))
		return result, false, fmt.Errorf("unexpected TS.43 event %T", result.Event)
	}
}

func (h *Handler) handleSMDSDiscovery(ctx context.Context, session *transferSession, transfer *activeTransfer, result *ts43.Result, event ts43.SMDSDiscoveryEvent) (*ts43.Result, error) {
	slog.Info("TS.43 transfer requires SM-DS discovery", "targetEID", event.TargetEID, "subscriptionResult", event.SubscriptionResult)
	cfg, err := smdsDownloadConfig(ctx, transfer.targetClient, event)
	if err != nil {
		slog.Warn("resolve TS.43 SM-DS download config", "error", err)
		return result, err
	}
	next, err := h.downloadEnableAndComplete(ctx, session, transfer, result, cfg)
	if err != nil {
		slog.Warn("download TS.43 SM-DS transferred profile", "error", err)
	}
	return next, err
}

func (h *Handler) handleSourceProfileDeletion(ctx context.Context, session *transferSession, transfer *activeTransfer, start transferStart, result *ts43.Result, event ts43.SourceProfileDeletionEvent) (*ts43.Result, error) {
	if err := session.confirmSourceDeletion(ctx, event.ICCID); err != nil {
		return result, err
	}
	session.sendIfConnected(transferServerMessage{Type: wsTypeProgress, Stage: transferStageDeleting})
	transfer.source.Close()
	if err := h.deleteSourceProfile(ctx, transfer.cfg, start, event.ICCID); err != nil {
		return result, err
	}
	return transfer.client.Continue(ctx, result, ts43.ContinueRequest{SourceProfileDeleted: true})
}

func ts43TransferDevice(imei string) ts43.Device {
	return ts43.Device{
		IMEI:            imei,
		Vendor:          ts43DeviceVendor,
		Model:           ts43DeviceModel,
		SoftwareVersion: ts43DeviceSoftwareVersion,
	}
}

func ts43SourceSIMType(profileType transferProfileType) ts43.SIMType {
	if profileType == transferProfilePhysical {
		return ts43.SIMTypePSIM
	}
	return ts43.SIMTypeESIM
}
