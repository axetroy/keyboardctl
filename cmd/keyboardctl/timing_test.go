package main

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// sampleDurations draws n values from a timing function for statistical checks.
func sampleDurations(n int, f func(r *rand.Rand) time.Duration) []float64 {
	r := rand.New(rand.NewSource(1))
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(f(r)) / float64(time.Millisecond)
	}
	return out
}

func mean(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// skewness returns the sample skewness.  A log-normal distribution has a
// positive skew; a uniform distribution is symmetric (skew ≈ 0), so this is
// what separates "human-like" timing from a flat random draw.
func skewness(xs []float64) float64 {
	n := float64(len(xs))
	m := mean(xs)
	var m2, m3 float64
	for _, x := range xs {
		d := x - m
		m2 += d * d
		m3 += d * d * d
	}
	m2 /= n
	m3 /= n
	if m2 == 0 {
		return 0
	}
	return m3 / math.Pow(m2, 1.5)
}

func TestHoldDuration_WithinBounds(t *testing.T) {
	for _, sc := range []ScanCode{
		ScancodeA, ScancodeSpace, ScancodeLShift, ScancodeF5,
		ScancodeLeft, ScancodeCapsLock, ScancodeKp7,
	} {
		p := holdParamsFor(sc)
		got := sampleDurations(2000, func(r *rand.Rand) time.Duration {
			return holdDuration(r, sc)
		})
		for _, ms := range got {
			if ms < p.min-0.5 || ms > p.max+0.5 {
				t.Fatalf("sc 0x%04X: hold %.1fms outside [%.0f,%.0f]", sc, ms, p.min, p.max)
			}
		}
	}
}

// TestHoldDuration_RightSkewed is the core property: the distribution must be
// right-skewed like real key presses, not symmetric like the old uniform draw.
func TestHoldDuration_RightSkewed(t *testing.T) {
	got := sampleDurations(20000, func(r *rand.Rand) time.Duration {
		return holdDuration(r, ScancodeA)
	})
	sk := skewness(got)
	if sk < 0.3 {
		t.Fatalf("hold-time skewness = %.3f, want >= 0.3 (right-skewed like hardware)", sk)
	}
	// The mean must sit above the median in a right-skewed distribution.
	med := holdParamsFor(ScancodeA).median
	if m := mean(got); m <= med {
		t.Fatalf("mean %.1f should exceed the median %.1f for a right-skewed hold", m, med)
	}
}

// TestHoldDuration_NotUniform rejects a flat distribution by checking that the
// values are concentrated near the low end rather than spread evenly.
func TestHoldDuration_NotUniform(t *testing.T) {
	p := holdParamsFor(ScancodeA)
	got := sampleDurations(20000, func(r *rand.Rand) time.Duration {
		return holdDuration(r, ScancodeA)
	})
	mid := (p.min + p.max) / 2
	var below int
	for _, ms := range got {
		if ms < mid {
			below++
		}
	}
	frac := float64(below) / float64(len(got))
	// A uniform draw would put ~50% below the midpoint; a log-normal with these
	// parameters puts the large majority there.
	if frac < 0.75 {
		t.Fatalf("only %.1f%% of holds below the midpoint, want >= 75%% (not uniform)", frac*100)
	}
}

// TestKeyClassesDiffer verifies that a modifier is held longer, on average,
// than an ordinary character, which is how real presses behave.
func TestKeyClassesDiffer(t *testing.T) {
	charMean := mean(sampleDurations(8000, func(r *rand.Rand) time.Duration {
		return holdDuration(r, ScancodeA)
	}))
	modMean := mean(sampleDurations(8000, func(r *rand.Rand) time.Duration {
		return holdDuration(r, ScancodeLShift)
	}))
	fnMean := mean(sampleDurations(8000, func(r *rand.Rand) time.Duration {
		return holdDuration(r, ScancodeF5)
	}))
	if modMean <= charMean {
		t.Fatalf("modifier mean %.1f should exceed character mean %.1f", modMean, charMean)
	}
	if fnMean >= charMean {
		t.Fatalf("function-key mean %.1f should be below character mean %.1f", fnMean, charMean)
	}
}

func TestClassOf(t *testing.T) {
	tests := []struct {
		sc   ScanCode
		want keyClass
	}{
		{ScancodeA, classChar},
		{ScancodeSpace, classChar},
		{Scancode1, classChar},
		{ScancodeLShift, classModifier},
		{ScancodeRCtrl, classModifier},
		{ScancodeLWin, classModifier},
		{ScancodeF1, classFunction},
		{ScancodeF12, classFunction},
		{ScancodeLeft, classFunction},
		{ScancodeKp7, classChar},
		{ScancodeCapsLock, classToggle},
		{ScancodeNumLock, classToggle},
	}
	for _, tt := range tests {
		if got := classOf(tt.sc); got != tt.want {
			t.Errorf("classOf(0x%04X) = %d, want %d", tt.sc, got, tt.want)
		}
	}
}

func TestFingerOf_HandsAndFingers(t *testing.T) {
	// Left-hand letters must map to 0-3 and right-hand letters to 4-7.
	for _, sc := range []ScanCode{ScancodeQ, ScancodeA, ScancodeZ, ScancodeW, ScancodeE, ScancodeR} {
		if f := fingerOf(sc); f < 0 || f > 3 {
			t.Errorf("fingerOf(0x%04X) = %d, want left-hand 0-3", sc, f)
		}
	}
	for _, sc := range []ScanCode{ScancodeY, ScancodeU, ScancodeI, ScancodeO, ScancodeP} {
		if f := fingerOf(sc); f < 4 || f > 7 {
			t.Errorf("fingerOf(0x%04X) = %d, want right-hand 4-7", sc, f)
		}
	}
	// A non-letter key has no finger assignment.
	if f := fingerOf(ScancodeEnter); f != -1 {
		t.Errorf("fingerOf(enter) = %d, want -1", f)
	}
}

func TestDigraphScale(t *testing.T) {
	// "ed" is same-finger (both left middle), so it must be slower than a
	// cross-hand pair such as "ep".
	if s := digraphScale(ScancodeE, ScancodeD); s != digraphSameFinger {
		t.Fatalf("digraphScale(e,d) = %.2f, want same-finger %.2f", s, digraphSameFinger)
	}
	if s := digraphScale(ScancodeE, ScancodeP); s != digraphCrossHand {
		t.Fatalf("digraphScale(e,p) = %.2f, want cross-hand %.2f", s, digraphCrossHand)
	}
	// Non-letter keys fall back to the neutral factor.
	if s := digraphScale(ScancodeEnter, ScancodeSpace); s != digraphUnknown {
		t.Fatalf("digraphScale(enter,space) = %.2f, want neutral %.2f", s, digraphUnknown)
	}
}

// TestInterKeyDelay_DigraphEffect verifies the pair-dependent timing: a
// same-finger transition is measurably slower than a cross-hand one.
func TestInterKeyDelay_DigraphEffect(t *testing.T) {
	slow := mean(sampleDurations(8000, func(r *rand.Rand) time.Duration {
		return interKeyDelay(r, ScancodeE, ScancodeD, 0)
	}))
	fast := mean(sampleDurations(8000, func(r *rand.Rand) time.Duration {
		return interKeyDelay(r, ScancodeE, ScancodeP, 0)
	}))
	if slow <= fast {
		t.Fatalf("same-finger gap %.1f should exceed cross-hand gap %.1f", slow, fast)
	}
}

// TestInterKeyDelay_NeverZero: even with no nominal delay, real fingers take
// time to travel, so the interval must have a positive floor.
func TestInterKeyDelay_NeverZero(t *testing.T) {
	for _, ms := range sampleDurations(5000, func(r *rand.Rand) time.Duration {
		return interKeyDelay(r, ScancodeA, ScancodeB, 0)
	}) {
		if ms < 10 {
			t.Fatalf("inter-key gap %.1fms is implausibly small", ms)
		}
	}
}

// TestInterKeyDelay_NominalDelayShiftsMedian: a larger -delay must move the
// distribution up, while keeping the spread.
func TestInterKeyDelay_NominalDelayShiftsMedian(t *testing.T) {
	base := mean(sampleDurations(8000, func(r *rand.Rand) time.Duration {
		return interKeyDelay(r, ScancodeA, ScancodeB, 50*time.Millisecond)
	}))
	big := mean(sampleDurations(8000, func(r *rand.Rand) time.Duration {
		return interKeyDelay(r, ScancodeA, ScancodeB, 200*time.Millisecond)
	}))
	if big <= base {
		t.Fatalf("200ms delay mean %.1f should exceed 50ms delay mean %.1f", big, base)
	}
}

func TestComboTransitionDelay_Bounds(t *testing.T) {
	for _, ms := range sampleDurations(5000, func(r *rand.Rand) time.Duration {
		return comboTransitionDelay(r)
	}) {
		if ms < 12 || ms > 90 {
			t.Fatalf("combo transition %.1fms outside [12,90]", ms)
		}
	}
}

// TestClassOfEdgeCases guards the explicit classification against the trap of
// range checks: several extended keys share a low byte with unrelated plain keys
// (Home 0xE047 vs keypad 7 = 0x47; keypad * = 0x37 vs Print Screen 0xE037).
func TestClassOfEdgeCases(t *testing.T) {
	for _, sc := range []ScanCode{
		ScancodeHome, ScancodeUp, ScancodeEnd, ScancodeDelete,
		ScancodeInsert, ScancodePrintScr, ScancodeF11, ScancodeF12,
	} {
		if got := classOf(sc); got != classFunction {
			t.Errorf("classOf(0x%04X) = %d, want classFunction(%d)", sc, got, classFunction)
		}
	}
	// Keypad keys are ordinary taps, including keypad * = 0x37, which a naive
	// 0x3B-0x44 range check would have missed.
	for _, sc := range []ScanCode{
		ScancodeKp0, ScancodeKp9, ScancodeKpPlus, ScancodeKpStar,
		ScancodeKpDot, ScancodeKpMinus, ScancodeKpEnter,
	} {
		if got := classOf(sc); got != classChar {
			t.Errorf("classOf(0x%04X) = %d, want classChar(%d)", sc, got, classChar)
		}
	}
	// Every class must have a distribution defined.
	for _, c := range []keyClass{classChar, classModifier, classFunction, classToggle} {
		if _, ok := holdByClass[c]; !ok {
			t.Errorf("no hold distribution for class %d", c)
		}
	}
}
