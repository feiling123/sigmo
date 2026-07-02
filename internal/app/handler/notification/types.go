package notification

type NotificationsResponse struct {
	SEs []NotificationGroupResponse `json:"ses"`
}

type NotificationGroupResponse struct {
	ID            string                 `json:"id"`
	Label         string                 `json:"label"`
	AID           string                 `json:"aid,omitempty"`
	EID           string                 `json:"eid,omitempty"`
	Notifications []NotificationResponse `json:"notifications"`
}

type NotificationResponse struct {
	SEID           string `json:"seId"`
	SELabel        string `json:"seLabel"`
	EID            string `json:"seEid,omitempty"`
	SequenceNumber string `json:"sequenceNumber"`
	ICCID          string `json:"iccid"`
	SMDP           string `json:"smdp"`
	Operation      string `json:"operation"`
}
