package outfitfeedback

import "testing"

func TestNormalizeFeedbackInput(t *testing.T) {
	note := "  useful \n"
	input, ok := normalize(Input{IssueTags: []string{"rainUnsuitable", "shoeDiscomfort"}, Note: &note})
	if !ok || input.Note == nil || *input.Note != "useful" || input.IssueTags[0] != "rainUnsuitable" {
		t.Fatal("valid feedback was not normalized")
	}
	cases := []Input{
		{},
		{IssueTags: []string{"rainUnsuitable", "rainUnsuitable"}},
		{IssueTags: []string{"unknown"}},
		{ThermalComfort: pointer("freezing")},
		{Note: pointer("bad\u0000text")},
	}
	for _, candidate := range cases {
		if _, ok := normalize(candidate); ok {
			t.Fatalf("invalid feedback accepted: %#v", candidate)
		}
	}
}

func pointer(value string) *string { return &value }
