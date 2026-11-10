package updater

import "strings"

type releaseVersion struct {
	core [3]string
	pre  []string
}

func digits(value string) bool {
	if value == "" {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func parseReleaseVersion(value string) (releaseVersion, bool) {
	var v releaseVersion
	value = strings.TrimPrefix(value, "v")
	value = strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(value, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) < 1 || len(core) > 3 {
		return v, false
	}
	for i := range v.core {
		v.core[i] = "0"
	}
	for i, p := range core {
		if !digits(p) {
			return v, false
		}
		v.core[i] = p
	}
	if len(parts) == 2 {
		v.pre = strings.Split(parts[1], ".")
		for _, p := range v.pre {
			if p == "" {
				return v, false
			}
			for _, c := range p {
				if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '-') {
					return v, false
				}
			}
		}
	}
	return v, true
}
func compareDigits(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
func compareReleaseVersions(a, b string) (int, bool) {
	left, ok := parseReleaseVersion(a)
	if !ok {
		return 0, false
	}
	right, ok := parseReleaseVersion(b)
	if !ok {
		return 0, false
	}
	for i := range left.core {
		if c := compareDigits(left.core[i], right.core[i]); c != 0 {
			return c, true
		}
	}
	if len(left.pre) == 0 && len(right.pre) == 0 {
		return 0, true
	}
	if len(left.pre) == 0 {
		return 1, true
	}
	if len(right.pre) == 0 {
		return -1, true
	}
	for i := 0; i < len(left.pre) && i < len(right.pre); i++ {
		x, y := left.pre[i], right.pre[i]
		nx, ny := digits(x), digits(y)
		var c int
		if nx && ny {
			c = compareDigits(x, y)
		} else if nx {
			c = -1
		} else if ny {
			c = 1
		} else {
			c = strings.Compare(x, y)
		}
		if c != 0 {
			return c, true
		}
	}
	if len(left.pre) < len(right.pre) {
		return -1, true
	}
	if len(left.pre) > len(right.pre) {
		return 1, true
	}
	return 0, true
}
