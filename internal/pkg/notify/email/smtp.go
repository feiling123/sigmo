package email

import (
	"fmt"
	"strings"

	"github.com/wneessen/go-mail"
)

type tlsPolicy struct {
	mail.TLSPolicy
}

func (p tlsPolicy) MarshalText() ([]byte, error) {
	switch p.TLSPolicy {
	case mail.TLSMandatory:
		return []byte("mandatory"), nil
	case mail.TLSOpportunistic:
		return []byte("opportunistic"), nil
	case mail.NoTLS:
		return []byte("none"), nil
	default:
		return nil, fmt.Errorf("unsupported email tls_policy: %d", p.TLSPolicy)
	}
}

func (p *tlsPolicy) UnmarshalText(text []byte) error {
	value := strings.ToLower(strings.TrimSpace(string(text)))
	switch value {
	case "", "mandatory":
		p.TLSPolicy = mail.TLSMandatory
		return nil
	case "opportunistic":
		p.TLSPolicy = mail.TLSOpportunistic
		return nil
	case "none", "notls", "no_tls":
		p.TLSPolicy = mail.NoTLS
		return nil
	default:
		return fmt.Errorf("unsupported email tls_policy: %q", text)
	}
}
