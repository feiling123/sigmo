//go:build esim_transfer

package esim

type TransferSourceResponse struct {
	Type               transferSourceType `json:"type"`
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Detail             string             `json:"detail,omitempty"`
	RequiresSourceIMEI bool               `json:"requiresSourceImei"`
}

type TransferSourcesResponse struct {
	Sources   []TransferSourceResponse `json:"sources"`
	CCIDError string                   `json:"ccidError,omitempty"`
}

type TransferProfilesRequest struct {
	SourceType transferSourceType `json:"sourceType"`
	SourceID   string             `json:"sourceId"`
	SourceIMEI string             `json:"sourceImei,omitempty"`
}

type TransferProfileResponse struct {
	ID                  string              `json:"id"`
	Type                transferProfileType `json:"type"`
	Name                string              `json:"name"`
	ServiceProviderName string              `json:"serviceProviderName,omitempty"`
	ICCID               string              `json:"iccid"`
	Icon                string              `json:"icon,omitempty"`
	RegionCode          string              `json:"regionCode,omitempty"`
	Enabled             bool                `json:"enabled"`
	Supported           bool                `json:"supported"`
	UnsupportedReason   string              `json:"unsupportedReason,omitempty"`
	CarrierName         string              `json:"carrierName,omitempty"`
}

type transferClientMessage struct {
	Type       string             `json:"type"`
	SourceType transferSourceType `json:"sourceType,omitempty"`
	SourceID   string             `json:"sourceId,omitempty"`
	ProfileID  string             `json:"profileId,omitempty"`
	SourceIMEI string             `json:"sourceImei,omitempty"`
	Accept     *bool              `json:"accept,omitempty"`
	Response   string             `json:"response,omitempty"`
}

type transferServerMessage struct {
	Type    string                    `json:"type"`
	Stage   string                    `json:"stage,omitempty"`
	Message string                    `json:"message,omitempty"`
	ICCID   string                    `json:"iccid,omitempty"`
	Input   *transferUserInputMessage `json:"input,omitempty"`
	Profile *downloadProfilePreview   `json:"profile,omitempty"`
}

type transferUserInputMessage struct {
	Text         string `json:"text"`
	AcceptLabel  string `json:"acceptLabel,omitempty"`
	RejectLabel  string `json:"rejectLabel,omitempty"`
	FreeText     bool   `json:"freeText"`
	FreeTextHint string `json:"freeTextHint,omitempty"`
}
