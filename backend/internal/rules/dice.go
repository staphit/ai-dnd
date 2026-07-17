package rules

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
)

// RandomSource matches the TS `() => number` random signature: a float in
// [0, 1). Keeping the float source (instead of a seeded-PRNG interface) lets
// the vitest fixtures (`rolls = [0.7, 0.5]; () => rolls.shift()`) port
// verbatim to Go tests.
type RandomSource func() float64

// DefaultRandom is the production random source.
func DefaultRandom() float64 { return rand.Float64() }

// Die rolls one die: floor(random()*sides)+1, matching combat.ts die().
func Die(sides int, random RandomSource) int {
	return int(math.Floor(random()*float64(sides))) + 1
}

var diceExpressionPattern = regexp.MustCompile(`(?i)^(\d+)d(\d+)(?:\s*([+-])\s*(\d+))?$`)

// ErrDiceRange mirrors the TS bounds error '傷害骰超出允許範圍'.
var ErrDiceRange = errors.New("傷害骰超出允許範圍")

// RollExpression parses and rolls an NdM(+/-K) expression, matching
// combat.ts rollExpression. critical doubles the die count before the
// bounds check, exactly like the TS source.
func RollExpression(expression string, random RandomSource, critical bool) (int, error) {
	match := diceExpressionPattern.FindStringSubmatch(strings.TrimSpace(expression))
	if match == nil {
		return 0, fmt.Errorf("無法辨識傷害骰：%s", expression)
	}
	count, _ := strconv.Atoi(match[1])
	if critical {
		count *= 2
	}
	sides, _ := strconv.Atoi(match[2])
	modifier := 0
	if match[4] != "" {
		modifier, _ = strconv.Atoi(match[4])
		if match[3] == "-" {
			modifier = -modifier
		}
	}
	if count < 1 || count > 40 || sides < 2 || sides > 100 {
		return 0, ErrDiceRange
	}
	total := modifier
	for i := 0; i < count; i++ {
		total += Die(sides, random)
	}
	if total < 0 {
		return 0, nil
	}
	return total, nil
}
