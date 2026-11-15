package main

import (
	"math"
	"math/rand"
	"time"
)

// This file models the timing of a human typing on a physical keyboard.  The
// goal is that the intervals between injected events have the same *statistical
// shape* as hardware-generated ones, not merely the same mean.
//
// Two properties matter, and neither is captured by a uniform distribution:
//
//   - Intervals are right-skewed.  Human key-hold and inter-key intervals are
//     well modelled by a log-normal distribution: most values cluster near a
//     floor (the shortest a finger can move) with a long tail.  A uniform draw
//     over the same range produces a flat histogram that no real device emits.
//
//   - Digraph timing depends on the key pair.  Letters typed by the same finger
//     or the same hand (e.g. "ed", "lo") come faster than ones that require a
//     hand or finger jump (e.g. "az", "pn"), so intervals are not independent.
//
// The numbers below are conservative mid-range values drawn from published
// keystroke-dynamics measurements; they are deliberately not tuned to mimic any
// particular person.

// keyClass groups keys whose physical press behaviour differs.  A key's class
// sets the parameters of its hold-time distribution.
type keyClass int

const (
	classChar     keyClass = iota // letters, digits, punctuation, space
	classModifier                 // Shift / Ctrl / Alt / Win — held, not tapped
	classFunction                 // F1-F12, arrows, navigation, keypad
	classToggle                   // Caps/Num/Scroll Lock — deliberately held
)

// holdParams describes a log-normal duration distribution in milliseconds.
//
//	duration = median * exp(sigma * N(0,1)),  clamped to [min, max]
//
// median is the typical value; sigma is the shape (higher = more spread).
// Real fingers are more consistent on modifiers (pressed deliberately) than on
// ordinary characters, hence the smaller sigma there.
type holdParams struct {
	median float64
	sigma  float64
	min    float64
	max    float64
}

var holdByClass = map[keyClass]holdParams{
	// Ordinary characters: ~70 ms median, a long tail when the finger lingers.
	classChar: {median: 70, sigma: 0.38, min: 30, max: 220},
	// Modifiers: pressed firmly and released quickly, but with low variance
	// because the whole hand is on the key rather than one finger reaching.
	classModifier: {median: 85, sigma: 0.30, min: 40, max: 240},
	// Function/navigation keys: short, decisive taps.
	classFunction: {median: 55, sigma: 0.35, min: 25, max: 180},
	// Lock toggles: people hold these noticeably longer.
	classToggle: {median: 95, sigma: 0.32, min: 45, max: 260},
}

// interKeyParams describes the gap between one key release and the next press.
//
// The floor is higher than a hold time: a finger has to travel to the next key,
// and that travel time is the dominant term, so the distribution is tighter.
// It is the same shape as a hold distribution, so it reuses holdParams.
type interKeyParams = holdParams

// digraphFactor scales the median gap by how easy the key pair is to type.
// Values below 1 are faster transitions (same hand, rolling motion); above 1
// are slower (same finger twice, or a hand jump).
const (
	digraphSameFinger = 1.35 // e.g. "ed" typed with one finger
	digraphSameHand   = 0.85 // e.g. "lo" — a natural roll
	digraphCrossHand  = 1.00 // opposite hands, the neutral case
	digraphUnknown    = 1.00
)

// logNormalSample returns a log-normal draw in milliseconds, clamped to
// [min, max].  rand is passed in so callers (and tests) can supply a source.
func logNormalSample(r *rand.Rand, p holdParams, scale float64) time.Duration {
	median := p.median * scale
	if median < p.min {
		median = p.min
	}
	// Exp of a normal deviate: the standard log-normal construction.
	ms := median * math.Exp(p.sigma*r.NormFloat64())
	if ms < p.min {
		ms = p.min
	}
	if ms > p.max {
		ms = p.max
	}
	return time.Duration(ms * float64(time.Millisecond))
}

// classOf reports which keyClass a scan code belongs to.
//
// The classification is deliberately explicit rather than range-based: several
// extended keys share their low byte with unrelated plain keys (Home 0xE047 vs
// keypad 7 = 0x47, keypad * = 0x37 vs Print Screen 0xE037), so byte ranges would
// misclassify them.
func classOf(sc ScanCode) keyClass {
	switch sc {
	case ScancodeLShift, ScancodeRShift,
		ScancodeLCtrl, ScancodeRCtrl,
		ScancodeLAlt, ScancodeRAlt,
		ScancodeLWin, ScancodeRWin:
		return classModifier

	case ScancodeCapsLock, ScancodeNumLock, ScancodeScrollLock:
		return classToggle

	case ScancodeF1, ScancodeF2, ScancodeF3, ScancodeF4, ScancodeF5,
		ScancodeF6, ScancodeF7, ScancodeF8, ScancodeF9, ScancodeF10,
		ScancodeF11, ScancodeF12,
		ScancodeHome, ScancodeEnd, ScancodeInsert, ScancodeDelete,
		ScancodeUp, ScancodeDown, ScancodeLeft, ScancodeRight,
		ScancodePageUp, ScancodePageDown, ScancodePrintScr:
		return classFunction

	default:
		// Letters, digits, punctuation, space and the keypad keys are ordinary
		// taps: their hold time does not depend on being on the numpad.
		return classChar
	}
}

// holdParamsFor returns the hold-time distribution for a key.
func holdParamsFor(sc ScanCode) holdParams {
	return holdByClass[classOf(sc)]
}

// fingerOf maps a scan code to a finger index (0-7) for digraph timing:
// 0 = left pinky, 1 = left ring, 2 = left middle, 3 = left index,
// 4 = right index, 5 = right middle, 6 = right ring, 7 = right pinky.
// -1 means "not a letter finger key" (digraph timing falls back to neutral).
func fingerOf(sc ScanCode) int {
	switch sc {
	case ScancodeQ, ScancodeA, ScancodeZ, Scancode1, ScancodeGrave:
		return 0
	case ScancodeW, ScancodeS, ScancodeX, Scancode2:
		return 1
	case ScancodeE, ScancodeD, ScancodeC, Scancode3:
		return 2
	case ScancodeR, ScancodeF, ScancodeV, ScancodeT, ScancodeG, ScancodeB,
		Scancode4, Scancode5:
		return 3
	case ScancodeY, ScancodeH, ScancodeN, ScancodeU, ScancodeJ, ScancodeM,
		Scancode6, Scancode7:
		return 4
	case ScancodeI, ScancodeK, ScancodeComma, Scancode8:
		return 5
	case ScancodeO, ScancodeL, ScancodeDot, Scancode9:
		return 6
	case ScancodeP, ScancodeSemicolon, ScancodeSlash, Scancode0,
		ScancodeMinus, ScancodeEqual, ScancodeLeftBrace, ScancodeRightBrace,
		ScancodeBackslash, ScancodeApostrophe:
		return 7
	}
	return -1
}

// digraphScale returns the median-gap multiplier for a from→to key transition.
func digraphScale(from, to ScanCode) float64 {
	f, t := fingerOf(from), fingerOf(to)
	if f < 0 || t < 0 {
		return digraphUnknown
	}
	if f == t {
		return digraphSameFinger
	}
	// Left-hand fingers are 0-3, right-hand are 4-7.
	if (f < 4) == (t < 4) {
		return digraphSameHand
	}
	return digraphCrossHand
}

// holdDuration returns how long a key is held down, in the shape of a real
// finger press.  r supplies randomness so the sequence is reproducible in tests.
func holdDuration(r *rand.Rand, sc ScanCode) time.Duration {
	return logNormalSample(r, holdParamsFor(sc), 1.0)
}

// interKeyDelay returns the gap between releasing `from` and pressing `to`.
//
// A non-zero nominal delay is treated as the *mean* the caller wants; the
// distribution around it keeps the same relative spread.  With delay == 0 the
// function still returns a realistic travel time, because a real keyboard never
// reports a release and the next press in the same instant.
func interKeyDelay(r *rand.Rand, from, to ScanCode, nominal time.Duration) time.Duration {
	p := interKeyParams{median: 110, sigma: 0.35, min: 25, max: 400}
	scale := digraphScale(from, to)

	if nominal > 0 {
		p.median = float64(nominal) / float64(time.Millisecond)
		// Keep a travel-time floor so a tiny -delay does not collapse to zero.
		p.min = math.Min(p.min, p.median*0.5)
	}
	return logNormalSample(r, p, scale)
}

// comboTransitionDelay returns the gap between the modifier going down and the
// main key following, and symmetrically on release.  Real hotkey presses are
// fast and tight; a large gap would look like two separate keystrokes.
func comboTransitionDelay(r *rand.Rand) time.Duration {
	p := holdParams{median: 35, sigma: 0.30, min: 12, max: 90}
	return logNormalSample(r, p, 1.0)
}
