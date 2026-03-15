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
	Query      string
	Response   string
	Sources    []Source
	IsFollowUp bool
	Error      string
}

type CodeBlock struct {
	Language string
	Content  string
}
