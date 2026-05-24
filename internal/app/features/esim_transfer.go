//go:build esim_transfer

package features

func init() {
	registered = append(registered, EsimTransfer)
}
