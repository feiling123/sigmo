package internet

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed apns.json
var apnJSON []byte

var defaultAPNs = mustDefaultAPNs(apnJSON)

type apnEntry struct {
	MCC      string `json:"mcc"`
	MNC      string `json:"mnc"`
	APN      string `json:"apn"`
	Protocol string `json:"protocol"`
	User     string `json:"user"`
	Password string `json:"pass"`
	AuthType *int   `json:"authType"`
}

type apnProfile struct {
	APN      string
	IPType   string
	Username string
	Password string
	Auth     string
}

type apnSelection struct {
	Requested          string
	Bearer             string
	Remembered         string
	OperatorIdentifier string
	DefaultAPNs        map[string]apnProfile
}

func mustDefaultAPNs(data []byte) map[string]apnProfile {
	apns, err := defaultAPNsFromJSON(data)
	if err != nil {
		panic(err)
	}
	return apns
}

func defaultAPNsFromJSON(data []byte) (map[string]apnProfile, error) {
	var entries []apnEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse apn json: %w", err)
	}

	apns := make(map[string]apnProfile)
	for _, entry := range entries {
		apn := strings.TrimSpace(entry.APN)
		mcc := strings.TrimSpace(entry.MCC)
		mnc := strings.TrimSpace(entry.MNC)
		if apn == "" || mcc == "" || mnc == "" {
			continue
		}
		operatorIdentifier := mcc + mnc
		if _, exists := apns[operatorIdentifier]; exists {
			continue
		}
		apns[operatorIdentifier] = apnProfile{
			APN:      apn,
			IPType:   androidProtocol(entry.Protocol),
			Username: strings.TrimSpace(entry.User),
			Password: entry.Password,
			Auth:     androidAuthType(entry.AuthType),
		}
	}
	return apns, nil
}

func selectAPN(selection apnSelection) string {
	if apn := firstAPN(selection.Requested, selection.Bearer, selection.Remembered); apn != "" {
		return apn
	}
	return defaultAPNFrom(selection.DefaultAPNs, selection.OperatorIdentifier)
}

func defaultAPNFrom(apns map[string]apnProfile, operatorIdentifier string) string {
	return defaultAPNProfileFrom(apns, operatorIdentifier).APN
}

func defaultAPNProfileFrom(apns map[string]apnProfile, operatorIdentifier string) apnProfile {
	profile := apns[strings.TrimSpace(operatorIdentifier)]
	profile.APN = strings.TrimSpace(profile.APN)
	profile.IPType = strings.ToLower(strings.TrimSpace(profile.IPType))
	profile.Username = strings.TrimSpace(profile.Username)
	profile.Auth = strings.ToLower(strings.TrimSpace(profile.Auth))
	return profile
}

func androidProtocol(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "IP":
		return "ipv4"
	case "IPV6":
		return "ipv6"
	case "IPV4V6":
		return "ipv4v6"
	default:
		return ""
	}
}

func firstAPN(values ...string) string {
	for _, value := range values {
		if apn := strings.TrimSpace(value); apn != "" {
			return apn
		}
	}
	return ""
}

func androidAuthType(value *int) string {
	if value == nil {
		return ""
	}
	switch *value {
	case 0:
		return "none"
	case 1:
		return "pap"
	case 2:
		return "chap"
	case 3:
		return "pap|chap"
	default:
		return ""
	}
}
