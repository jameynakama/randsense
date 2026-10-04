package oewn_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
)

// xmlWrap wraps a snippet of one or more <LexicalEntry> elements in the
// minimal LexicalResource/Lexicon scaffolding the streaming parser
// expects. Use this in table-driven tests so each case is one entry's
// worth of XML, not a full document.
func xmlWrap(entries string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<LexicalResource>
  <Lexicon id="oewn" label="test" language="en" email="x@y" license="cc-by-4.0" version="test">
` + entries + `
  </Lexicon>
</LexicalResource>`
}

func collect(t *testing.T, body string) []oewn.Entry {
	t.Helper()
	var got []oewn.Entry
	err := oewn.Parse(strings.NewReader(xmlWrap(body)), func(e oewn.Entry) error {
		got = append(got, e)
		return nil
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return got
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []oewn.Entry
	}{
		{
			"noun with irregular plural",
			`
			<LexicalEntry id="oewn-goose-n">
			  <Lemma writtenForm="goose" partOfSpeech="n">
			    <Pronunciation>ˈɡuːs</Pronunciation>
			  </Lemma>
			  <Form writtenForm="geese"/>
			  <Sense id="oewn-goose__1.05.00.." synset="oewn-01858313-n"/>
			</LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "goose", POS: "n", Forms: []string{"geese"}, Frames: nil}},
		},
		{
			"noun without Form",
			`
			<LexicalEntry id="oewn-shrimp-n">
		      <Lemma writtenForm="shrimp" partOfSpeech="n"/>
		      <Sense id="oewn-shrimp__1.05.00.." synset="oewn-02314320-n"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "shrimp", POS: "n", Forms: nil, Frames: nil}},
		},
		{
			"verb single sense single subcat code",
			`
			<LexicalEntry id="oewn-sleep-v">
				<Lemma writtenForm="sleep" partOfSpeech="v"/>
				<Sense id="oewn-sleep__2.29.00.." subcat="vita" synset="oewn-00018651-v"/>
			</LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "sleep", POS: "v", Forms: nil, Frames: []string{"vita"}}},
		},
		{
			"verb single sense multiple subcat codes",
			`
			<LexicalEntry id="oewn-devour-v">
		      <Lemma writtenForm="devour" partOfSpeech="v"/>
		      <Sense id="oewn-devour__2.34.00.." subcat="vtaa vtai" synset="oewn-01172275-v"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "devour", POS: "v", Forms: nil, Frames: []string{"vtaa", "vtai"}}},
		},
		{
			"verb multiple senses overlapping codes (dedup)",
			`
			<LexicalEntry id="oewn-goose-v">
		      <Lemma writtenForm="goose" partOfSpeech="v">
		        <Pronunciation>ˈɡuːs</Pronunciation>
		      </Lemma>
		      <Sense id="oewn-goose__2.35.00.." subcat="vtaa vtai" synset="oewn-01459708-v"/>
		      <Sense id="oewn-goose__2.35.02.." subcat="vtaa" synset="oewn-01233625-v"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "goose", POS: "v", Forms: nil, Frames: []string{"vtaa", "vtai"}}},
		},
		{
			"verb with no subcat",
			`
		    <LexicalEntry id="oewn-loiter-v">
		      <Lemma writtenForm="loiter" partOfSpeech="v"/>
		      <Sense id="oewn-loiter__2.38.00.." synset="oewn-02061425-v"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "loiter", POS: "v", Forms: nil, Frames: nil}},
		},
		{
			"adjective head (a)",
			`
		    <LexicalEntry id="oewn-good-a">
		      <Lemma writtenForm="good" partOfSpeech="a"/>
		      <Sense id="oewn-good__3.00.00.." synset="oewn-00231927-a"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "good", POS: "a", Forms: nil, Frames: nil}},
		},
		{
			"adjective satellite (s)",
			`
		    <LexicalEntry id="oewn-effervescent-s">
		      <Lemma writtenForm="effervescent" partOfSpeech="s"/>
		      <Sense id="oewn-effervescent__5.00.00.." synset="oewn-02212345-s"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "effervescent", POS: "s", Forms: nil, Frames: nil}},
		},
		{
			"cardinal number",
			`
		    <LexicalEntry id="oewn-lxxiii-a">
		      <Lemma writtenForm="lxxiii" partOfSpeech="a"/>
		      <Sense id="oewn-lxxiii__5.00.00.cardinal.00" synset="oewn-02201083-s"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "lxxiii", POS: "a", Cardinal: true}},
		},
		{
			"adverb (r)",
			`
		    <LexicalEntry id="oewn-curly-r">
		      <Lemma writtenForm="curly" partOfSpeech="r"/>
		      <Sense id="oewn-curly__4.02.00.." synset="oewn-00120000-r"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "curly", POS: "r", Forms: nil, Frames: nil}},
		},
		{
			"multi-word lemma with space",
			`
		    <LexicalEntry id="oewn-sea_anemone-n">
		      <Lemma writtenForm="sea anemone" partOfSpeech="n"/>
		      <Sense id="oewn-sea_anemone__1.05.00.." synset="oewn-02316707-n"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "sea anemone", POS: "n", Forms: nil, Frames: nil}},
		},
		{
			"lemma with apostrophe entity",
			`
		    <LexicalEntry id="oewn-oclock-r">
		      <Lemma writtenForm="o&apos;clock" partOfSpeech="r"/>
		      <Sense id="oewn-oclock__4.02.00.." synset="oewn-00010000-r"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "o'clock", POS: "r", Forms: nil, Frames: nil}},
		},
		{
			"lemma with Pronunciation child",
			`
		    <LexicalEntry id="oewn-goose-v">
		      <Lemma writtenForm="goose" partOfSpeech="v">
		        <Pronunciation>ˈɡuːs</Pronunciation>
		      </Lemma>
		      <Sense id="oewn-goose__2.35.00.." subcat="vtaa vtai" synset="oewn-01459708-v"/>
		      <Sense id="oewn-goose__2.35.02.." subcat="vtaa" synset="oewn-01233625-v"/>
		    </LexicalEntry>
			`,
			[]oewn.Entry{{Lemma: "goose", POS: "v", Forms: nil, Frames: []string{"vtaa", "vtai"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collect(t, tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestParseSampleFile(t *testing.T) {
	path := filepath.Join("testdata", "sample.xml")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var got []oewn.Entry
	err = oewn.Parse(f, func(e oewn.Entry) error {
		got = append(got, e)
		return nil
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	const expectedElements = 22
	if len(got) != expectedElements {
		t.Errorf("expected %d; got %d: %#v", expectedElements, len(got), got)
	}

	find := func(lemma, pos string) (oewn.Entry, bool) {
		for _, e := range got {
			if e.Lemma == lemma && e.POS == pos {
				return e, true
			}
		}
		return oewn.Entry{}, false
	}

	checks := []struct {
		name   string
		lemma  string
		pos    string
		forms  []string
		frames []string
	}{
		{"irregular plural extracted", "goose", "n", []string{"geese"}, nil},
		{"regular noun has no Form", "shrimp", "n", nil, nil},
		{"proper noun case preserved", "Microsoft", "n", nil, nil},
		{"multi-word lemma keeps internal space", "sea anemone", "n", nil, nil},
		{"verb subcat deduped across senses, order preserved", "goose", "v", nil, []string{"vtaa", "vtai"}},
		{"adjective satellite POS not collapsed", "effervescent", "s", nil, nil},
		{"apostrophe entity decoded in lemma", "o'clock", "r", nil, nil},
		{"leading-apostrophe lemma still emitted", "'hood", "n", nil, nil},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			e, ok := find(tc.lemma, tc.pos)
			if !ok {
				t.Fatalf("no entry for lemma=%q pos=%q", tc.lemma, tc.pos)
			}
			if !reflect.DeepEqual(e.Forms, tc.forms) {
				t.Errorf("Forms: got %#v, want %#v", e.Forms, tc.forms)
			}
			if !reflect.DeepEqual(e.Frames, tc.frames) {
				t.Errorf("Frames: got %#v, want %#v", e.Frames, tc.frames)
			}
		})
	}
}
