package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/engine"
)

// SpecSection is the OpenSpec summary rendered at the top of a merge request
// body, above the change summary.
//
// Two channels carry the spec, because they answer different questions. The
// spec directory is committed into the branch, giving a reviewable diff and a
// permanent record. This section is rendered inline, so a reviewer can decide
// at a glance without navigating to files. With the pipeline running
// unattended by default, the MR is the first place a human sees what the agent
// decided — so a missing spec is reported loudly rather than silently omitted.
type SpecSection struct {
	Slug         string
	Goal         string
	KeyDecisions []string
	VerifySteps  []string
	// Missing is set when no spec directory was produced. The MR still opens
	// — working code must not be held hostage to a missing document — but the
	// body says so explicitly.
	Missing bool
}

// specSectionCharBudget caps the rendered section so a long spec cannot push
// the actual change summary below the fold in a PR view.
const specSectionCharBudget = 6000

// BuildSpecSection reads the spec set at specDir and extracts the parts a
// reviewer decides from. Parsing is best-effort by design: the spec is written
// by a CLI agent following a template, not a machine-checked format, so a
// heading it phrased differently degrades that one subsection to empty rather
// than failing the MR.
func BuildSpecSection(slug, specDir string) SpecSection {
	section := SpecSection{Slug: slug}

	stat, err := os.Stat(specDir)
	if err != nil || !stat.IsDir() {
		section.Missing = true
		return section
	}

	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			return ""
		}
		return string(b)
	}

	// A Small task writes spec.md; Medium/Large split across proposal.md and
	// specs.md. Read whichever exist and take the first non-empty match, so
	// both shapes render the same way.
	proposal := read("proposal.md") + "\n" + read("spec.md")
	specs := read("specs.md") + "\n" + read("spec.md")

	section.Goal = firstNonEmpty(
		extractMDSection(proposal, "Goal"),
		extractMDSection(proposal, "Problem"),
	)
	section.KeyDecisions = firstNonEmptyList(
		extractMDListItems(proposal, "Decisions"),
		extractMDListItems(proposal, "Key Decisions"),
	)
	section.VerifySteps = firstNonEmptyList(
		extractMDListItems(specs, "Verify"),
		extractMDListItems(read("tasks.md"), "Verify"),
	)

	if section.Goal == "" && len(section.KeyDecisions) == 0 && len(section.VerifySteps) == 0 {
		// The directory exists but nothing parseable came out of it. That is
		// the same problem for a reviewer as no spec at all, so report it the
		// same way rather than rendering an empty, reassuring-looking block.
		section.Missing = true
	}
	return section
}

// Render produces the markdown block prepended to the MR body. Every string
// that reaches it is agent-authored from repo content, so it is redacted on
// the way out — the same treatment captured CLI output already gets.
func (s SpecSection) Render() string {
	var sb strings.Builder
	sb.WriteString("## 📋 OpenSpec\n\n")

	if s.Missing {
		sb.WriteString("⚠️ No OpenSpec was produced for this task. Review the diff directly — there is no stated intent to check it against.\n\n---\n\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Spec set: `docs/openspecs/%s/` (committed in this branch)\n\n", s.Slug))
	if s.Goal != "" {
		sb.WriteString("**Goal**\n\n" + s.Goal + "\n\n")
	}
	if len(s.KeyDecisions) > 0 {
		sb.WriteString("**Key decisions**\n\n")
		for _, d := range s.KeyDecisions {
			sb.WriteString("- " + d + "\n")
		}
		sb.WriteString("\n")
	}
	if len(s.VerifySteps) > 0 {
		sb.WriteString("**How to verify**\n\n")
		for _, v := range s.VerifySteps {
			sb.WriteString("- " + v + "\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("---\n\n")

	out := sb.String()
	if len(out) > specSectionCharBudget {
		out = out[:specSectionCharBudget] + "\n\n_(spec summary truncated — read the full set in the branch)_\n\n---\n\n"
	}
	return engine.RedactSecrets(out)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNonEmptyList(vals ...[]string) []string {
	for _, v := range vals {
		if len(v) > 0 {
			return v
		}
	}
	return nil
}
