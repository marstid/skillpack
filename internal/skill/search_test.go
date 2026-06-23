package skill

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
)

func searchDemoFS() fstest.MapFS {
	return fstest.MapFS{
		"logs-triage/SKILL.md":       &fstest.MapFile{Data: []byte("---\nname: logs-triage\ndescription: Triage Datadog logs and investigate errors.\n---\nbody\n")},
		"metrics-query/SKILL.md":     &fstest.MapFile{Data: []byte("---\nname: metrics-query\ndescription: Aggregate and query metric time series.\n---\nbody\n")},
		"apm-root-cause/SKILL.md":    &fstest.MapFile{Data: []byte("---\nname: apm-root-cause\ndescription: Root cause APM traces and spans.\n---\nbody\n")},
		"synthetics-runner/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: synthetics-runner\ndescription: Run and inspect synthetic tests.\n---\nbody\n")},
	}
}

func mustStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(context.Background(), nil, searchDemoFS())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestSearch_EmptyQueryUnsorted(t *testing.T) {
	s := mustStore(t)
	got := s.Search("")
	want := s.List()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i].Name != want[i].Name {
			t.Errorf("index %d: got %q, want %q (expected insertion order preserved)", i, got[i].Name, want[i].Name)
		}
	}
}

func TestSearch_Subsequence(t *testing.T) {
	s := mustStore(t)
	// "lg" is a subsequence of "logs-triage" (L-o-G) but nothing else.
	got := s.Search("lg")
	if len(got) == 0 || got[0].Name != "logs-triage" {
		t.Errorf("expected logs-triage first for 'lg', got %+v", got)
	}
	// "apm" should match only apm-root-cause.
	got = s.Search("apm")
	if len(got) != 1 || got[0].Name != "apm-root-cause" {
		t.Errorf("expected only apm-root-cause for 'apm', got %+v", got)
	}
}

func TestSearch_Ranking(t *testing.T) {
	s := mustStore(t)
	// "log" matches "logs-triage" (name prefix) and "metrics-query" via
	// description "...query metric...". The name-prefix match should rank
	// first.
	got := s.Search("log")
	if len(got) == 0 {
		t.Fatal("expected matches for 'log'")
	}
	if got[0].Name != "logs-triage" {
		t.Errorf("expected logs-triage to rank first for 'log', got %+v", got)
	}
	// "synth" should match synthetics-runner (description "synthetic tests")
	// and nothing else; it's a discriminating subsequence (only "synthetic"
	// contains the y-n-t-h run).
	got = s.Search("synth")
	if len(got) != 1 || got[0].Name != "synthetics-runner" {
		t.Errorf("expected only synthetics-runner for 'synth', got %+v", got)
	}
}

func TestSearch_NoMatch(t *testing.T) {
	s := mustStore(t)
	if got := s.Search("zzzzzzz"); len(got) != 0 {
		t.Errorf("expected no matches, got %+v", got)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	s := mustStore(t)
	upper := s.Search("APM")
	lower := s.Search("apm")
	if len(upper) != len(lower) || (len(upper) > 0 && upper[0].Name != lower[0].Name) {
		t.Errorf("APM and apm should produce same results; upper=%+v lower=%+v", upper, lower)
	}
}

func TestSearch_StableTieOrder(t *testing.T) {
	// Skills that share a description prefix so two rank approximately
	// equally; verify stable insertion order is preserved for ties.
	fsys := fstest.MapFS{
		"a-skill/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: a-skill\ndescription: shared keyword.\n---\nbody\n")},
		"b-skill/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: b-skill\ndescription: shared keyword.\n---\nbody\n")},
		"c-skill/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: c-skill\ndescription: shared keyword.\n---\nbody\n")},
	}
	s, err := New(context.Background(), nil, fsys)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got := s.Search("shared")
	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %+v", got)
	}
	wantOrder := []string{"a-skill", "b-skill", "c-skill"}
	for i, want := range wantOrder {
		if got[i].Name != want {
			t.Errorf("tie at index %d: got %q, want %q (stable order not preserved)", i, got[i].Name, want)
		}
	}
}

func TestSubsequenceScore_Helper(t *testing.T) {
	cases := []struct {
		hay, q string
		wantOk bool
	}{
		{"logs-triage", "log", true},
		{"logs-triage", "lg", true},
		{"logs-triage", "lg", true},
		{"logs-triage", "zx", false},
		{"metric query", "mq", true},
		{"", "", true},
		{"anything", "", true},
	}
	for _, c := range cases {
		_, ok := subsequenceScore(strings.ToLower(c.hay), strings.ToLower(c.q))
		if ok != c.wantOk {
			t.Errorf("subsequenceScore(%q,%q) ok = %v, want %v", c.hay, c.q, ok, c.wantOk)
		}
	}
}
