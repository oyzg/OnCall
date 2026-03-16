package retrieval

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	knowledgeDomain "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/domain"
)

type Reference struct {
	DocumentID    string  `json:"document_id"`
	DocumentTitle string  `json:"document_title"`
	Category      string  `json:"category"`
	ChunkIndex    int     `json:"chunk_index"`
	Chunk         string  `json:"chunk"`
	Score         float64 `json:"score"`
}

type RetrieveOptions struct {
	Limit    int    `json:"limit"`
	Category string `json:"category"`
}

type Report struct {
	Query          string      `json:"query"`
	Answer         string      `json:"answer"`
	References     []Reference `json:"references"`
	ScannedDocs    int         `json:"scanned_docs"`
	ScannedChunks  int         `json:"scanned_chunks"`
	MatchedChunks  int         `json:"matched_chunks"`
	Strategy       string      `json:"strategy"`
	RequestedLimit int         `json:"requested_limit"`
}

type Service struct {
	knowledge *knowledgeApp.Service
}

func NewService(knowledge *knowledgeApp.Service) *Service {
	return &Service{knowledge: knowledge}
}

func (s *Service) Retrieve(user authDomain.User, query string, limit int) []Reference {
	report := s.RetrieveWithOptions(user, query, RetrieveOptions{Limit: limit})
	return report.References
}

func (s *Service) RetrieveWithOptions(user authDomain.User, query string, options RetrieveOptions) Report {
	query = strings.TrimSpace(query)
	limit := options.Limit
	if limit <= 0 {
		limit = 4
	}

	report := Report{
		Query:          query,
		Answer:         BuildAnswer(query, nil),
		References:     nil,
		Strategy:       "precomputed_text_chunks",
		RequestedLimit: limit,
	}
	if query == "" {
		return report
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return report
	}

	chunks := s.knowledge.ListReadyChunks(user, strings.TrimSpace(options.Category))
	report.ScannedChunks = len(chunks)

	scannedDocs := make(map[string]struct{})
	hits := make([]Reference, 0)
	for _, chunk := range chunks {
		scannedDocs[chunk.DocumentID] = struct{}{}
		score := similarityScore(queryTokens, tokenize(chunk.Content))
		if score <= 0 {
			continue
		}

		hits = append(hits, Reference{
			DocumentID:    chunk.DocumentID,
			DocumentTitle: chunk.DocumentTitle,
			Category:      chunk.Category,
			ChunkIndex:    chunk.Index,
			Chunk:         chunk.Content,
			Score:         score,
		})
	}

	report.ScannedDocs = len(scannedDocs)
	report.MatchedChunks = len(hits)

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			if hits[i].DocumentTitle == hits[j].DocumentTitle {
				return hits[i].ChunkIndex < hits[j].ChunkIndex
			}
			return hits[i].DocumentTitle < hits[j].DocumentTitle
		}
		return hits[i].Score > hits[j].Score
	})

	hits = deduplicate(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}

	report.References = hits
	report.Answer = BuildAnswer(query, hits)
	return report
}

func BuildAnswer(question string, references []Reference) string {
	if len(references) == 0 {
		return "当前知识库没有命中直接相关的片段。建议补充更具体的服务名、错误码或故障现象，再重新检索。"
	}

	var builder strings.Builder
	builder.WriteString("基于知识库检索，当前最相关的信息如下：\n\n")
	for _, reference := range references {
		builder.WriteString("- [")
		builder.WriteString(reference.DocumentTitle)
		builder.WriteString("] ")
		builder.WriteString(reference.Chunk)
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString("问题：")
	builder.WriteString(strings.TrimSpace(question))
	builder.WriteString("\n")
	builder.WriteString("建议先按以上引用片段排查，如果仍不明确，再继续联动工具查询日志、指标或历史告警。")

	return builder.String()
}

func ToMessageReferences(references []Reference) []knowledgeDomain.Reference {
	items := make([]knowledgeDomain.Reference, 0, len(references))
	for _, reference := range references {
		items = append(items, knowledgeDomain.Reference{
			DocumentID:    reference.DocumentID,
			DocumentTitle: reference.DocumentTitle,
			Category:      reference.Category,
			Excerpt:       reference.Chunk,
			Score:         reference.Score,
		})
	}
	return items
}

func tokenize(content string) []string {
	normalized := strings.ToLower(strings.TrimSpace(content))
	var builder strings.Builder
	for _, r := range normalized {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.In(r, unicode.Han):
			builder.WriteRune(r)
		default:
			builder.WriteRune(' ')
		}
	}

	parts := strings.Fields(builder.String())
	tokens := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		tokens = append(tokens, part)
		tokens = append(tokens, expandToken(part)...)
	}
	if len(tokens) > 0 {
		return deduplicateTokens(tokens)
	}

	return expandToken(normalized)
}

func similarityScore(queryTokens, chunkTokens []string) float64 {
	if len(queryTokens) == 0 || len(chunkTokens) == 0 {
		return 0
	}

	chunkSet := make(map[string]struct{}, len(chunkTokens))
	for _, token := range chunkTokens {
		chunkSet[token] = struct{}{}
	}

	matches := 0
	for _, token := range queryTokens {
		if _, ok := chunkSet[token]; ok || hasPartialMatch(token, chunkTokens) {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}

	lengthPenalty := math.Log(float64(max(utf8.RuneCountInString(strings.Join(chunkTokens, "")), 2)))
	return float64(matches) / math.Max(lengthPenalty, 1)
}

func expandToken(token string) []string {
	runes := []rune(strings.TrimSpace(token))
	if len(runes) == 0 {
		return nil
	}

	items := make([]string, 0, len(runes)*2)
	if len(runes) <= 4 {
		items = append(items, string(runes))
	}

	for _, r := range runes {
		if unicode.In(r, unicode.Han) || unicode.IsNumber(r) {
			items = append(items, string(r))
		}
	}

	for size := 2; size <= min(4, len(runes)); size++ {
		for start := 0; start+size <= len(runes); start++ {
			items = append(items, string(runes[start:start+size]))
		}
	}

	return deduplicateTokens(items)
}

func deduplicateTokens(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	items := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		items = append(items, token)
	}
	return items
}

func hasPartialMatch(queryToken string, chunkTokens []string) bool {
	for _, token := range chunkTokens {
		if strings.Contains(token, queryToken) || strings.Contains(queryToken, token) {
			return true
		}
	}
	return false
}

func deduplicate(hits []Reference) []Reference {
	seen := make(map[string]struct{}, len(hits))
	items := make([]Reference, 0, len(hits))
	for _, hit := range hits {
		key := hit.DocumentID + "|" + hit.Chunk
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, hit)
	}
	return items
}
