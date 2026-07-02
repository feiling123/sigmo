package euicc

type SASUPResponse struct {
	Name   string `json:"name"`
	Region string `json:"region,omitempty"`
}

type SEsResponse struct {
	SEs []SEItemResponse `json:"ses"`
}

type SEItemResponse struct {
	ID           string        `json:"id"`
	Label        string        `json:"label"`
	AID          string        `json:"aid,omitempty"`
	EID          string        `json:"eid,omitempty"`
	FreeSpace    int32         `json:"freeSpace,omitempty"`
	SASUP        SASUPResponse `json:"sasUp,omitempty"`
	Certificates []string      `json:"certificates,omitempty"`
}
