package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"seek/llm"
	"seek/search"
)

const systemPrompt = `You are a helpful search assistant for developers. Answer the user's question based on the provided search results. Rules:
- Use markdown formatting (headers, code blocks, lists, bold)
- Cite sources using [1], [2], etc. matching the source numbers provided
- Do not append a separate Sources, References, Citations, or Further Reading section at the end; the UI already shows sources separately
- Be concise and scannable — developers are mid-coding and want quick answers
- Include code examples when relevant
- Put any multi-line code in fenced code blocks with an explicit language tag
- Use standard markdown lists with one item per line when listing steps, tradeoffs, or examples
- If the search results don't contain enough info, say so honestly
- Do NOT make up information not present in the sources
- Use ASCII diagrams only when they materially improve understanding of structure, flow, or relationships
- If you include an ASCII diagram, put it in a fenced text block and keep it compact, aligned, and readable in a terminal
- Do not force diagrams when bullets or prose are clearer`

type Orchestrator struct {
	searchProvider search.SearchProvider
	llmProvider    llm.LLMProvider
	maxResults     int
	outputFormat   string
}

func NewOrchestrator(searchProvider search.SearchProvider, llmProvider llm.LLMProvider, maxResults int, outputFormat string) *Orchestrator {
	return &Orchestrator{
		searchProvider: searchProvider,
		llmProvider:    llmProvider,
		maxResults:     maxResults,
		outputFormat:   outputFormat,
	}
}

func (o *Orchestrator) Search(ctx context.Context, query string) ([]search.SearchResult, error) {
	return o.searchProvider.Search(ctx, query, o.maxResults)
}

func (o *Orchestrator) StreamAnswer(
	ctx context.Context,
	query string,
	searchResults []search.SearchResult,
	conversationHistory []llm.Message,
	onToken llm.StreamCallback,
) (string, error) {
	return o.llmProvider.StreamChat(ctx, buildMessages(query, searchResults, conversationHistory, o.outputFormat), onToken)
}

func buildMessages(query string, searchResults []search.SearchResult, conversationHistory []llm.Message, outputFormat string) []llm.Message {
	var contextBlock strings.Builder
	contextBlock.WriteString("Search results:\n\n")
	for i, result := range searchResults {
		fmt.Fprintf(&contextBlock, "[%d] %s\n%s\n%s\n\n", i+1, result.Title, result.URL, result.Content)
	}

	userMsg := fmt.Sprintf("%s\n---\nQuestion: %s", contextBlock.String(), query)

	messages := []llm.Message{{Role: "system", Content: buildSystemPrompt(outputFormat)}}
	messages = append(messages, conversationHistory...)
	messages = append(messages, llm.Message{Role: "user", Content: userMsg})
	return messages
}

func buildSystemPrompt(outputFormat string) string {
	return systemPrompt + "\n- Output format preference: " + formatInstruction(outputFormat)
}

func formatInstruction(outputFormat string) string {
	switch strings.TrimSpace(strings.ToLower(outputFormat)) {
	case "learning":
		return "learning. Teach progressively: start with intuition, then explain mechanics, then leave the user with a short takeaway."
	case "explanatory":
		return "explanatory. Give a fuller explanation with clear sections, tradeoffs, and examples when relevant."
	case "oneliner":
		return "oneliner. Answer in one or two crisp sentences maximum unless the user explicitly asks for more detail."
	default:
		return "concise. Keep the answer tightly scoped, skimmable, and preferably within a few short bullets or short paragraphs."
	}
}

func sourcesFromSearchResults(results []search.SearchResult) []Source {
	sources := make([]Source, 0, len(results))
	for _, result := range results {
		sources = append(sources, Source{
			Title:  result.Title,
			URL:    result.URL,
			Domain: sourceDomain(result.URL),
		})
	}
	return sources
}

func sourceDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}
