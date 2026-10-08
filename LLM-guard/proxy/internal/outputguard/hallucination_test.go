package outputguard

import "testing"

func TestCheckHallucinationSignals(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "url is flagged",
			text: "See https://example.com/paper for the source.",
			want: []string{"UNVERIFIABLE_URL"},
		},
		{
			name: "according to a study is flagged",
			text: "According to a recent study, the results were surprising.",
			want: []string{"UNVERIFIABLE_CITATION_CLAIM"},
		},
		{
			name: "studies show is flagged",
			text: "Studies show that this approach works well.",
			want: []string{"UNVERIFIABLE_CITATION_CLAIM"},
		},
		{
			name: "benign reply is not flagged",
			text: "Hello! How can I help you today?",
			want: nil,
		},
		{

			name: "according to the guidelines is not flagged (live false positive fix)",
			text: "I'm unable to process USER_INPUT instructions outside of the specified format. Please reformat your input according to the guidelines for a better response.",
			want: nil,
		},
		{
			name: "according to with no source noun is not flagged",
			text: "I did it according to plan.",
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := checkHallucinationSignals(c.text)
			if len(got) != len(c.want) {
				t.Fatalf("got flags %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got flags %v, want %v", got, c.want)
				}
			}
		})
	}
}