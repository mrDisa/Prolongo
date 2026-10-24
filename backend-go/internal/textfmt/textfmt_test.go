package textfmt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEUR(t *testing.T) {
	cases := map[int64]string{
		0:         "€0,00",
		5:         "€0,05",
		360:       "€3,60",
		28657:     "€286,57",
		469765:    "€4 697,65",
		703267:    "€7 032,67",
		123456789: "€1 234 567,89",
		-1050:     "-€10,50",
	}
	for cents, want := range cases {
		assert.Equal(t, want, EUR(cents), "%d", cents)
	}
}

func TestDays(t *testing.T) {
	cases := map[float64]string{
		394.94:  "395 дн.",
		-39.98:  "40 дн.",
		2:       "2 дн.",
		0.99:    "1 дн.",
		0.96:    "23 ч",
		0.0368:  "1 ч",
		0.5:     "12 ч",
		0.01:    "14 мин",
		0.00001: "1 мин",
	}
	for d, want := range cases {
		assert.Equal(t, want, Days(d), "%v", d)
	}
}

func TestPlural(t *testing.T) {
	forms := func(n int) string { return Plural(n, "день", "дня", "дней") }
	cases := map[int]string{1: "день", 2: "дня", 4: "дня", 5: "дней", 11: "дней", 12: "дней", 14: "дней", 21: "день", 22: "дня", 30: "дней", 111: "дней", 101: "день"}
	for n, want := range cases {
		assert.Equal(t, want, forms(n), "%d", n)
	}
}
