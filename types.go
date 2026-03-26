package main

type AppState int

const (
	StateLoading AppState = iota
	StateViewing
	StateSources
	StateInput
	StateSearchInput
	StateCodeSelect
)

func (s AppState) String() string {
	switch s {
	case StateLoading:
		return "loading"
	case StateViewing:
		return "viewing"
	case StateSources:
		return "sources"
	case StateInput:
		return "input"
	case StateSearchInput:
		return "search"
	case StateCodeSelect:
		return "code-select"
	default:
		return "unknown"
	}
}

type Source struct {
	Title  string
	URL    string
	Domain string
}

type Turn struct {
	Query         string
	SearchQuery   string
	Response      string
	Sources       []Source
	AttachedFiles []AttachedFile
	IsFollowUp    bool
	Error         string
	HistoryID     *int64
}

type CodeBlock struct {
	Language string
	Content  string
}

type SearchTiming struct {
	SearchMs int64
	LLMMs    int64
	TotalMs  int64
}
