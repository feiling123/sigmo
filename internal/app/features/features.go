package features

import "slices"

const EsimTransfer = "esimTransfer"

var registered []string

func List() []string {
	out := slices.Clone(registered)
	slices.Sort(out)
	return out
}
