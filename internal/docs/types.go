package docs

type Type struct {
	Name     string
	Emoji    string
	Sections []string
}

var Types = []Type{
	{"investigation", "🔍", []string{"Question", "Findings", "Options considered", "Recommendation", "Open questions", "For future work"}},
	{"plan", "📋", []string{"Goal", "Approach", "Steps", "Risks", "Out of scope", "For future work"}},
	{"summary", "✅", []string{"What changed", "Why", "How to verify", "Caveats", "Follow-ups", "For future work"}},
	{"bug", "🐛", []string{"Symptom", "Root cause", "Fix", "How to verify", "Related areas at risk", "For future work"}},
	{"how-to", "🛠️", []string{"When to use", "Steps", "Troubleshooting"}},
	{"reference", "📖", []string{"Overview", "Details", "Gotchas"}},
}

var Statuses = []string{"in-progress", "done", "abandoned", "superseded", "regressed"}

func LookupType(name string) (Type, bool) {
	for _, t := range Types {
		if t.Name == name {
			return t, true
		}
	}
	return Type{}, false
}

func TypeNames() []string {
	var names []string
	for _, t := range Types {
		names = append(names, t.Name)
	}
	return names
}
