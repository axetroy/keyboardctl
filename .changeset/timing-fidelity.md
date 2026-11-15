---
"keyboardctl": minor
---

Make injected keystroke timing statistically match real hardware, and document
what the software approach can and cannot hide:

- New `cmd/keyboardctl/timing.go`: key-hold and inter-key intervals are drawn
  from **log-normal** distributions instead of a flat uniform range. Real typing
  is right-skewed, so a uniform histogram was itself a detectable signature.
- Hold time now depends on the **key class** (character / modifier / function /
  toggle), because a held Shift behaves differently from a tapped letter.
- Inter-key intervals model the **digraph effect**: same-finger pairs (`ed`) are
  slower than same-hand rolls (`lo`), so intervals are pair-correlated rather
  than independent.
- `timing.go` is platform-independent, so its statistical tests run on any OS,
  including the Linux CI runner.
- README: new "timing model and fidelity" section, including the architectural
  fingerprints that a filter-driver design cannot remove (test signing,
  `UpperFilters`, the loaded module, the device stack).
