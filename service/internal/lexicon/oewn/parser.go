// Package oewn parses Open English WordNet's WN-LMF XML release.
//
// The 2025 release file is ~80MB uncompressed with ~136k LexicalEntry
// elements, so the parser streams: it does not load the full document
// into memory. Callers use Parse with a yield callback; each entry is
// decoded individually and discarded.
package oewn

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// Entry is the parser's output -- one row's worth of randsense-relevant
// data extracted from a single OEW <LexicalEntry>. All filtering and
// per-POS dispatch happens at the ingest layer, not here.
type Entry struct {
	// Lemma is the surface form, OEW-canonical. Multi-word entries use
	// internal spaces ("sea anemone"). XML entities are already decoded
	// by encoding/xml, so this is plain Go string content.
	Lemma string

	// POS is the OEW partOfSpeech code: "n" | "v" | "a" | "s" | "r".
	// "s" (adjective satellite) is mapped to the adjectives table at
	// ingest -- the parser does not collapse it.
	POS string

	// Forms is the inflected surface forms from <Form> children. In
	// practice only nouns populate this (irregular plurals); verbs and
	// adj/adv almost never have <Form> elements in OEW.
	Forms []string

	// Frames is the union of subcat codes across all <Sense> children
	// of this entry, deduplicated, original first-seen order preserved.
	// Populated only for verbs. Each <Sense subcat="..."> may contain
	// multiple space-separated codes; this field flattens them.
	Frames []string

	// Cardinal reports a sense that is a cardinal number ("lxxiii", "seven"),
	// whose sense ID is headed by "cardinal".
	Cardinal bool
}

// lexicalEntry mirrors the XML structure of <LexicalEntry> for decoding.
// Children of <Lemma> like <Pronunciation> are ignored automatically --
// only the attributes we declare are populated.
type lexicalEntry struct {
	Lemma struct {
		WrittenForm  string `xml:"writtenForm,attr"`
		PartOfSpeech string `xml:"partOfSpeech,attr"`
	} `xml:"Lemma"`
	Forms []struct {
		WrittenForm string `xml:"writtenForm,attr"`
	} `xml:"Form"`
	Senses []struct {
		ID     string `xml:"id,attr"`
		Subcat string `xml:"subcat,attr"`
	} `xml:"Sense"`
}

// Parse streams r as WN-LMF XML, invoking yield for each <LexicalEntry>.
// If yield returns a non-nil error, Parse stops and returns that error.
//
// Memory use stays O(entry size), not O(file size): the decoder advances
// through the document one token at a time, and each entry's struct is
// released after yield returns.
func Parse(r io.Reader, yield func(Entry) error) error {
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "LexicalEntry" {
			continue
		}

		var le lexicalEntry
		if err := dec.DecodeElement(&le, &se); err != nil {
			return err
		}
		if err := yield(toEntry(le)); err != nil {
			return err
		}
	}
}

// toEntry collapses a parsed lexicalEntry into the public Entry type.
//
// Responsibilities:
//   - Flatten <Form> children into Entry.Forms.
//   - Deduplicate subcat codes across all <Sense> children, splitting
//     each attribute on whitespace (strings.Fields handles empty
//     gracefully). Preserve first-seen order so output is stable.
//
// Does NOT filter the lemma or normalize case -- that's the ingest
// layer's job.
func toEntry(le lexicalEntry) Entry {
	var forms []string
	for _, f := range le.Forms {
		forms = append(forms, f.WrittenForm)
	}

	var frames []string
	seen := map[string]bool{}
	cardinal := false
	for _, s := range le.Senses {
		cardinal = cardinal || strings.Contains(s.ID, ".cardinal.")
		for f := range strings.FieldsSeq(s.Subcat) {
			if !seen[f] {
				frames = append(frames, f)
				seen[f] = true
			}
		}
	}

	return Entry{
		Lemma:    le.Lemma.WrittenForm,
		POS:      le.Lemma.PartOfSpeech,
		Forms:    forms,
		Frames:   frames,
		Cardinal: cardinal,
	}
}
