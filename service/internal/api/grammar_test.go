package api_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/grammar"
)

const labeledGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "Verb"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[phrase.S]
label = "sentence"
description = "A complete thought."

[phrase.NP]
label = "noun phrase"
description = "Names a thing."

[slot.Determiner]
label = "determiner"
description = "Points at a noun."

[slot.Noun]
label = "noun"
description = "A thing."

[slot.Verb]
label = "verb"
description = "Says what happens."
`

func TestGrammarServesRulesAndLabels(t *testing.T) {
	srv := newServer(t, api.RouterConfig{Grammar: loadGrammar(t, labeledGrammar)})
	defer srv.Close()

	resp := call(t, srv, http.MethodGet, "/api/v1/grammar", "", nil)
	var got grammar.Description
	decode(t, resp, http.StatusOK, &got)

	want := grammar.Description{
		Start: "S",
		Phrases: map[string]grammar.Phrase{
			"S":  {Label: grammar.Label{Label: "sentence", Description: "A complete thought."}, Rules: [][]string{{"NP", "Verb"}}},
			"NP": {Label: grammar.Label{Label: "noun phrase", Description: "Names a thing."}, Rules: [][]string{{"Determiner", "Noun"}}},
		},
		Slots: map[string]grammar.Label{
			"Determiner": {Label: "determiner", Description: "Points at a noun."},
			"Noun":       {Label: "noun", Description: "A thing."},
			"Verb":       {Label: "verb", Description: "Says what happens."},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("grammar:\n got %+v\nwant %+v", got, want)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}
