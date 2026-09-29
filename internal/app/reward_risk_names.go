package app

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

func normalizeRiskName(name string) string {
	name = cases.Fold().String(norm.NFKC.String(name))
	var out strings.Builder
	count := 0
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.In(r, unicode.Cf) {
			continue
		}
		out.WriteRune(r)
		count++
		if count == 100 {
			break
		}
	}
	return norm.NFKC.String(out.String())
}

func riskNameTemplate(name string) string {
	runes := []rune(name)
	for _, hex := range []bool{true, false} {
		i := len(runes)
		digit := false
		for i > 0 {
			c := runes[i-1]
			if c >= '0' && c <= '9' {
				digit = true
				i--
				continue
			}
			if hex && c >= 'a' && c <= 'f' {
				i--
				continue
			}
			break
		}
		minimum := 2
		if hex {
			minimum = 8
		}
		if i >= 4 && len(runes)-i >= minimum && digit {
			return string(runes[:i]) + func() string {
				if hex {
					return "#hex"
				}
				return "#number"
			}()
		}
	}
	return ""
}

func riskNameSimilarity(a, b string) float64 {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	if a == b {
		return 1
	}
	if len(ar) < 6 || len(br) < 6 {
		return 0
	}
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ac := range ar {
		current := make([]int, len(br)+1)
		current[0] = i + 1
		for j, bc := range br {
			cost := 1
			if ac == bc {
				cost = 0
			}
			current[j+1] = min(current[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = current
	}
	return 1 - float64(prev[len(br)])/float64(max(len(ar), len(br)))
}
