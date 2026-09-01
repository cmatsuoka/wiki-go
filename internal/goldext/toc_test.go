package goldext

import (
	"strings"
	"testing"
)

func TestCollectHeadings(t *testing.T) {
	t.Run("basic headings", func(t *testing.T) {
		md := "# One\n## Two\n### Three\n"
		hs := CollectHeadings(md)
		if len(hs) != 3 {
			t.Fatalf("expected 3 headings, got %d", len(hs))
		}
		if hs[0].Level != 1 || hs[0].Text != "One" || hs[0].ID != "one" {
			t.Errorf("unexpected first heading: %+v", hs[0])
		}
		if hs[1].Level != 2 || hs[1].ID != "two" {
			t.Errorf("unexpected second heading: %+v", hs[1])
		}
		if hs[2].Level != 3 || hs[2].ID != "three" {
			t.Errorf("unexpected third heading: %+v", hs[2])
		}
	})

	t.Run("skips fenced code blocks", func(t *testing.T) {
		md := "# Real\n```\n# Fake\n```\n## Also Real\n"
		hs := CollectHeadings(md)
		if len(hs) != 2 {
			t.Fatalf("expected 2 headings, got %d: %+v", len(hs), hs)
		}
		if hs[0].Text != "Real" || hs[1].Text != "Also Real" {
			t.Errorf("unexpected headings: %+v", hs)
		}
	})

	t.Run("respects explicit ID", func(t *testing.T) {
		md := "# Title {#custom-id}\n"
		hs := CollectHeadings(md)
		if len(hs) != 1 || hs[0].ID != "custom-id" {
			t.Errorf("expected custom-id, got %+v", hs)
		}
	})

	t.Run("deduplicates identical slugs", func(t *testing.T) {
		md := "# Same\n## Same\n### Same\n"
		hs := CollectHeadings(md)
		if len(hs) != 3 {
			t.Fatalf("expected 3 headings, got %d", len(hs))
		}
		if hs[0].ID != "same" || hs[1].ID != "same-1" || hs[2].ID != "same-2" {
			t.Errorf("unexpected IDs: %s, %s, %s", hs[0].ID, hs[1].ID, hs[2].ID)
		}
	})

	t.Run("deduplicates colliding explicit IDs", func(t *testing.T) {
		md := "# A {#foo}\n## B {#foo}\n"
		hs := CollectHeadings(md)
		if len(hs) != 2 {
			t.Fatalf("expected 2 headings, got %d", len(hs))
		}
		if hs[0].ID != "foo" || hs[1].ID != "foo-1" {
			t.Errorf("unexpected IDs: %s, %s", hs[0].ID, hs[1].ID)
		}
	})
}

func TestTocPreprocessor_ReplacesMarker(t *testing.T) {
	md := "# One\n[toc]\n## Two\n"
	out := TocPreprocessor(md, "")
	if !strings.Contains(out, `class="wiki-toc`) {
		t.Errorf("expected TOC HTML in output, got:\n%s", out)
	}
	if !strings.Contains(out, `href="#one"`) || !strings.Contains(out, `href="#two"`) {
		t.Errorf("expected TOC links to #one and #two, got:\n%s", out)
	}
}

func TestTocPreprocessor_InjectsHeadingIDs(t *testing.T) {
	md := "# Hello World\n## Sub Section\n"
	out := TocPreprocessor(md, "")
	if !strings.Contains(out, "# Hello World {#hello-world}") {
		t.Errorf("expected injected ID on first heading, got:\n%s", out)
	}
	if !strings.Contains(out, "## Sub Section {#sub-section}") {
		t.Errorf("expected injected ID on second heading, got:\n%s", out)
	}
}

func TestTocPreprocessor_CollidingExplicitIDsMatchTOC(t *testing.T) {
	// Regression test: when two headings have colliding explicit IDs,
	// CollectHeadings deduplicates the second to "foo-1". The rewritten
	// heading line must use the deduplicated ID so the rendered anchor
	// matches the TOC link.
	md := "# A {#foo}\n[toc]\n## B {#foo}\n"
	out := TocPreprocessor(md, "")

	// TOC links
	if !strings.Contains(out, `href="#foo"`) {
		t.Errorf("expected TOC link href=#foo, got:\n%s", out)
	}
	if !strings.Contains(out, `href="#foo-1"`) {
		t.Errorf("expected TOC link href=#foo-1, got:\n%s", out)
	}

	// Heading anchors — both must resolve to the deduplicated IDs
	if !strings.Contains(out, "# A {#foo}") {
		t.Errorf("expected first heading with {#foo}, got:\n%s", out)
	}
	if !strings.Contains(out, "## B {#foo-1}") {
		t.Errorf("expected second heading rewritten to {#foo-1}, got:\n%s", out)
	}
}

func TestTocPreprocessor_SkipsCodeBlocks(t *testing.T) {
	md := "# Real\n```\n# Fake\n[toc]\n```\n"
	out := TocPreprocessor(md, "")
	if strings.Contains(out, `class="wiki-toc`) {
		t.Errorf("expected no TOC generated for marker inside code block, got:\n%s", out)
	}
	if !strings.Contains(out, "# Real {#real}") {
		t.Errorf("expected real heading to receive ID, got:\n%s", out)
	}
	if strings.Contains(out, "# Fake {#fake}") {
		t.Errorf("expected heading inside code block to be untouched, got:\n%s", out)
	}
}

func TestGenerateTOCHTML_Empty(t *testing.T) {
	out := GenerateTOCHTML(nil)
	if !strings.Contains(out, "No headings found") {
		t.Errorf("expected empty-state message, got: %s", out)
	}
}

func TestGenerateTOCHTML_Nesting(t *testing.T) {
	hs := []Heading{
		{Level: 1, Text: "A", ID: "a"},
		{Level: 2, Text: "A1", ID: "a1"},
		{Level: 1, Text: "B", ID: "b"},
	}
	out := GenerateTOCHTML(hs)
	if !strings.Contains(out, `href="#a"`) || !strings.Contains(out, `href="#a1"`) || !strings.Contains(out, `href="#b"`) {
		t.Errorf("expected all headings linked, got: %s", out)
	}
	if !strings.Contains(out, `class="toc-list"`) {
		t.Errorf("expected top-level list class, got: %s", out)
	}
}
