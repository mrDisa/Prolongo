// Package textfmt formats numbers for the Russian texts of reasons and explanations.
package textfmt

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// EUR formats cents as "€4 697,65": space between thousands, comma before cents.
func EUR(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	euros := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, r := range euros {
		if i > 0 && (len(euros)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s€%s,%02d", sign, b.String(), cents%100)
}

// Days formats a length of time given in days, rounded to a readable unit:
// "395 дн." from one day up, "3 ч" below a day, "45 мин" below an hour.
func Days(d float64) string {
	d = math.Abs(d)
	switch hours := d * 24; {
	case math.Round(d) >= 1 && hours >= 23.5:
		return fmt.Sprintf("%.0f дн.", math.Round(d))
	case math.Round(hours) >= 1:
		return fmt.Sprintf("%.0f ч", math.Round(hours))
	default:
		return fmt.Sprintf("%.0f мин", math.Max(1, math.Round(hours*60)))
	}
}

// Plural picks the Russian word form for n: Plural(1, "день", "дня", "дней") = "день",
// 3 → "дня", 5 → "дней", 11 → "дней", 21 → "день".
func Plural(n int, one, few, many string) string {
	n %= 100
	if n < 0 {
		n = -n
	}
	switch {
	case n >= 11 && n <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	default:
		return many
	}
}
