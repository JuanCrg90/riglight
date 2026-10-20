package systemmetrics

import (
	"errors"
	"regexp"
	"sort"
)

var interfaceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,14}$`)

// ValidateInterfaces accepts the explicitly configured private uplink set.
func ValidateInterfaces(names []string) ([]string, error) {
	if len(names) < 1 || len(names) > 8 {
		return nil, errors.New("invalid interface selection")
	}
	out := append([]string(nil), names...)
	sort.Strings(out)
	for index, name := range out {
		if name == "lo" || !interfaceName.MatchString(name) || (index > 0 && name == out[index-1]) {
			return nil, errors.New("invalid interface selection")
		}
	}
	return out, nil
}
