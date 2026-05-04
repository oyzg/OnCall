package retrieval

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/oyzg/OnCall/backend/go-api/internal/ai/gateway"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	knowledgeDomain "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/domain"
)

type Reference struct {
	DocumentID    string   `json:"document_id"`
	DocumentTitle string   `json:"document_title"`
	Category      string   `json:"category"`
	ChunkIndex    int      `json:"chunk_index"`
	Chunk         string   `json:"chunk"`
	Score         float64  `json:"score"`
	LexicalScore  float64  `json:"lexical_score"`
	SemanticScore float64  `json:"semantic_score"`
	BoostScore    float64  `json:"boost_score"`
	MatchReasons  []string `json:"match_reasons,omitempty"`
}

type RetrieveOptions struct {
	Limit    int    `json:"limit"`
	Category string `json:"category"`
}

type Report struct {
	Query              string      `json:"query"`
	RewrittenQuery     string      `json:"rewritten_query"`
	QueryTerms         []string    `json:"query_terms"`
	ExpandedTerms      []string    `json:"expanded_terms"`
	Answer             string      `json:"answer"`
	References         []Reference `json:"references"`
	ScannedDocs        int         `json:"scanned_docs"`
	ScannedChunks      int         `json:"scanned_chunks"`
	MatchedChunks      int         `json:"matched_chunks"`
	LexicalCandidates  int         `json:"lexical_candidates"`
	SemanticCandidates int         `json:"semantic_candidates"`
	RerankedChunks     int         `json:"reranked_chunks"`
	Strategy           string      `json:"strategy"`
	RequestedLimit     int         `json:"requested_limit"`
	EmbeddingBackend   string      `json:"embedding_backend,omitempty"`
	VectorBackend      string      `json:"vector_backend,omitempty"`
	LexicalBackend     string      `json:"lexical_backend,omitempty"`
}

type Service struct {
	knowledge *knowledgeApp.Service
	remote    RemoteRetriever
}

type RemoteRetriever interface {
	RetrieveKnowledge(ctx context.Context, request gateway.RAGRetrieveRequest) (gateway.RAGRetrieveResponse, error)
}

type queryPlan struct {
	Raw        string
	Normalized string
	Terms      []string
	Expanded   []string
	CharTerms  []string
	Rewritten  string
}

type scoredChunk struct {
	Chunk         knowledgeApp.Chunk
	LexicalScore  float64
	SemanticScore float64
	BoostScore    float64
	Score         float64
	Reasons       []string
}

func NewService(knowledge *knowledgeApp.Service) *Service {
	return &Service{knowledge: knowledge}
}

func (s *Service) SetRemoteRetriever(remote RemoteRetriever) {
	s.remote = remote
}

func (s *Service) Retrieve(user authDomain.User, query string, limit int) []Reference {
	report := s.RetrieveWithOptions(user, query, RetrieveOptions{Limit: limit})
	return report.References
}

func (s *Service) RetrieveWithOptions(user authDomain.User, query string, options RetrieveOptions) Report {
	if s.remote != nil {
		remoteReport, err := s.remote.RetrieveKnowledge(context.Background(), gateway.RAGRetrieveRequest{
			Query:    strings.TrimSpace(query),
			Category: strings.TrimSpace(options.Category),
			Limit:    options.Limit,
		})
		if err == nil {
			return fromGatewayReport(remoteReport)
		}
	}

	plan := buildQueryPlan(query)
	limit := options.Limit
	if limit <= 0 {
		limit = 4
	}

	report := Report{
		Query:          strings.TrimSpace(query),
		RewrittenQuery: plan.Rewritten,
		QueryTerms:     plan.Terms,
		ExpandedTerms:  plan.Expanded,
		Answer:         BuildAnswer(query, nil),
		Strategy:       "hybrid_lexical_semantic_rerank",
		RequestedLimit: limit,
		EmbeddingBackend: "local_fallback",
		VectorBackend:    "local_fallback",
		LexicalBackend:   "local_fallback",
	}
	if plan.Normalized == "" || len(plan.Expanded) == 0 {
		return report
	}

	chunks := s.knowledge.ListReadyChunks(user, strings.TrimSpace(options.Category))
	report.ScannedChunks = len(chunks)
	if len(chunks) == 0 {
		return report
	}

	docSet := make(map[string]struct{}, len(chunks))
	df := make(map[string]int, len(chunks)*4)
	for _, chunk := range chunks {
		docSet[chunk.DocumentID] = struct{}{}
		seen := make(map[string]struct{}, len(chunk.Terms))
		for _, term := range chunk.Terms {
			if _, exists := seen[term]; exists {
				continue
			}
			seen[term] = struct{}{}
			df[term]++
		}
	}
	report.ScannedDocs = len(docSet)

	scored := make([]scoredChunk, 0, len(chunks))
	for _, chunk := range chunks {
		item := scoreChunk(plan, chunk, df, len(chunks))
		if item.Score <= 0 {
			continue
		}
		if item.LexicalScore > 0 {
			report.LexicalCandidates++
		}
		if item.SemanticScore > 0 {
			report.SemanticCandidates++
		}
		scored = append(scored, item)
	}

	report.MatchedChunks = len(scored)
	if len(scored) == 0 {
		return report
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			if scored[i].Chunk.DocumentTitle == scored[j].Chunk.DocumentTitle {
				return scored[i].Chunk.Index < scored[j].Chunk.Index
			}
			return scored[i].Chunk.DocumentTitle < scored[j].Chunk.DocumentTitle
		}
		return scored[i].Score > scored[j].Score
	})

	references := rerank(scored, limit)
	report.RerankedChunks = len(references)
	report.References = references
	report.Answer = BuildAnswer(query, references)
	return report
}

func fromGatewayReport(remote gateway.RAGRetrieveResponse) Report {
	references := make([]Reference, 0, len(remote.References))
	for _, reference := range remote.References {
		references = append(references, Reference{
			DocumentID:    reference.DocumentID,
			DocumentTitle: reference.DocumentTitle,
			Category:      reference.Category,
			ChunkIndex:    reference.ChunkIndex,
			Chunk:         reference.Chunk,
			Score:         reference.Score,
			LexicalScore:  reference.LexicalScore,
			SemanticScore: reference.SemanticScore,
			BoostScore:    reference.BoostScore,
			MatchReasons:  reference.MatchReasons,
		})
	}

	return Report{
		Query:              remote.Query,
		RewrittenQuery:     remote.RewrittenQuery,
		QueryTerms:         remote.QueryTerms,
		ExpandedTerms:      remote.ExpandedTerms,
		Answer:             remote.Answer,
		References:         references,
		ScannedDocs:        remote.ScannedDocs,
		ScannedChunks:      remote.ScannedChunks,
		MatchedChunks:      remote.MatchedChunks,
		LexicalCandidates:  remote.LexicalCandidates,
		SemanticCandidates: remote.SemanticCandidates,
		RerankedChunks:     remote.RerankedChunks,
		Strategy:           remote.Strategy,
		RequestedLimit:     remote.RequestedLimit,
		EmbeddingBackend:   remote.EmbeddingBackend,
		VectorBackend:      remote.VectorBackend,
		LexicalBackend:     remote.LexicalBackend,
	}
}

func BuildAnswer(question string, references []Reference) string {
	if len(references) == 0 {
		return "当前知识库没有命中直接相关的片段。建议补充更具体的服务名、错误码、指标名或故障现象，再重新检索。"
	}

	var builder strings.Builder
	builder.WriteString("基于混合检索，当前最相关的信息如下：\n\n")
	for _, reference := range references {
		builder.WriteString("- [")
		builder.WriteString(reference.DocumentTitle)
		builder.WriteString("] ")
		builder.WriteString(reference.Chunk)
		builder.WriteString("\n")
	}

	builder.WriteString("\n问题：")
	builder.WriteString(strings.TrimSpace(question))
	builder.WriteString("\n建议先按以上引用片段排查，再结合工具查询日志、指标与历史告警继续缩小范围。")
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

func buildQueryPlan(query string) queryPlan {
	normalized := normalizeText(query)
	terms := uniqueNonEmpty(strings.Fields(normalized))
	expanded := append([]string{}, terms...)
	expanded = append(expanded, phraseExpand(normalized)...)
	for _, term := range terms {
		expanded = append(expanded, expandQueryToken(term)...)
		expanded = append(expanded, synonymExpand(term)...)
	}

	expanded = uniqueNonEmpty(expanded)
	charTerms := charTerms(normalized)
	return queryPlan{
		Raw:        query,
		Normalized: normalized,
		Terms:      terms,
		Expanded:   expanded,
		CharTerms:  charTerms,
		Rewritten:  strings.Join(expanded, " "),
	}
}

func normalizeText(content string) string {
	var builder strings.Builder
	for _, r := range []rune(strings.ToLower(strings.TrimSpace(content))) {
		switch {
		case r == '\n', r == '\r', r == '\t':
			builder.WriteRune(' ')
		case strings.ContainsRune("[](){}<>:;,./!?@#$%^&*-_=+|\"'`~", r):
			builder.WriteRune(' ')
		default:
			builder.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func expandQueryToken(token string) []string {
	runes := []rune(strings.TrimSpace(token))
	if len(runes) == 0 {
		return nil
	}

	items := make([]string, 0, len(runes)*2)
	for _, r := range runes {
		if r > 127 || (r >= '0' && r <= '9') {
			items = append(items, string(r))
		}
	}
	for size := 2; size <= min(4, len(runes)); size++ {
		for start := 0; start+size <= len(runes); start++ {
			items = append(items, string(runes[start:start+size]))
		}
	}
	return items
}

func synonymExpand(term string) []string {
	table := map[string][]string{
		"sop":        {"runbook", "procedure"},
		"p95":        {"latency", "slow", "timeout", "延迟", "耗时"},
		"latency":    {"p95", "timeout", "slow", "延迟", "耗时"},
		"timeout":    {"latency", "slow", "超时", "延迟"},
		"error":      {"5xx", "exception", "failure", "错误", "异常"},
		"5xx":        {"error", "exception", "failure", "错误"},
		"failure":    {"error", "exception", "异常", "失败"},
		"release":    {"deploy", "发布", "上线"},
		"deploy":     {"release", "发布", "上线"},
		"database":   {"db", "mysql", "连接池", "数据库"},
		"db":         {"database", "mysql", "数据库"},
		"mysql":      {"database", "db", "数据库"},
		"grafana":    {"alert", "dashboard", "监控"},
		"prometheus": {"metrics", "alert", "监控"},
		"告警":         {"异常", "故障", "报警", "alert"},
		"异常":         {"故障", "错误", "告警"},
		"故障":         {"异常", "排障", "告警"},
		"排障":         {"故障", "分析", "定位"},
		"延迟":         {"耗时", "timeout", "latency"},
		"耗时":         {"延迟", "timeout", "latency"},
		"错误":         {"异常", "5xx", "error"},
		"异常率":        {"错误率", "5xx", "error ratio"},
		"错误率":        {"异常率", "5xx", "error ratio"},
	}
	return table[term]
}

func phraseExpand(normalized string) []string {
	rules := map[string][]string{
		"卡了": {"延迟", "latency", "timeout", "慢", "变慢"},
		"变慢": {"延迟", "latency", "timeout", "卡顿"},
		"很慢": {"延迟", "latency", "timeout", "卡顿"},
		"超时": {"timeout", "延迟", "latency"},
		"卡顿": {"延迟", "latency", "timeout", "变慢"},
		"用户服务": {"user", "service", "user service", "user-service", "auth", "authentication"},
		"登录接口": {"login", "auth", "authentication", "user-service"},
		"登录":  {"login", "auth", "authentication"},
	}

	items := make([]string, 0, 8)
	for phrase, expanded := range rules {
		if strings.Contains(normalized, phrase) {
			items = append(items, expanded...)
		}
	}
	return uniqueNonEmpty(items)
}

func scoreChunk(plan queryPlan, chunk knowledgeApp.Chunk, df map[string]int, totalChunks int) scoredChunk {
	lexicalScore, lexicalReasons := lexicalScore(plan, chunk, df, totalChunks)
	semanticScore, semanticReasons := semanticScore(plan, chunk)
	boostScore, boostReasons := boostScore(plan, chunk)

	score := lexicalScore*0.58 + semanticScore*0.30 + boostScore*0.12
	if score < 0.12 {
		return scoredChunk{}
	}

	reasons := append([]string{}, lexicalReasons...)
	reasons = append(reasons, semanticReasons...)
	reasons = append(reasons, boostReasons...)

	return scoredChunk{
		Chunk:         chunk,
		LexicalScore:  lexicalScore,
		SemanticScore: semanticScore,
		BoostScore:    boostScore,
		Score:         score,
		Reasons:       uniqueNonEmpty(reasons),
	}
}

func lexicalScore(plan queryPlan, chunk knowledgeApp.Chunk, df map[string]int, totalChunks int) (float64, []string) {
	if len(plan.Expanded) == 0 || len(chunk.Terms) == 0 {
		if strings.TrimSpace(chunk.DocumentTitle) == "" {
			return 0, nil
		}
	}

	chunkTerms := make(map[string]struct{}, len(chunk.Terms))
	for _, term := range chunk.Terms {
		chunkTerms[term] = struct{}{}
	}
	for _, term := range titleTerms(normalizeText(chunk.DocumentTitle)) {
		chunkTerms[term] = struct{}{}
	}

	matchedWeight := 0.0
	totalWeight := 0.0
	matched := make([]string, 0, 4)
	for _, term := range plan.Expanded {
		weight := 1.0 + math.Log(float64(totalChunks+1)/float64(df[term]+1))
		totalWeight += weight
		if _, ok := chunkTerms[term]; ok {
			matchedWeight += weight
			if len(matched) < 4 {
				matched = append(matched, term)
			}
		}
	}

	if matchedWeight == 0 || totalWeight == 0 {
		return 0, nil
	}
	return matchedWeight / totalWeight, []string{"lexical:" + strings.Join(matched, ",")}
}

func semanticScore(plan queryPlan, chunk knowledgeApp.Chunk) (float64, []string) {
	if len(plan.CharTerms) == 0 || len(chunk.CharTerms) == 0 {
		return 0, nil
	}

	chunkCharTerms := make(map[string]struct{}, len(chunk.CharTerms))
	for _, term := range chunk.CharTerms {
		chunkCharTerms[term] = struct{}{}
	}

	matched := 0
	for _, term := range plan.CharTerms {
		if _, ok := chunkCharTerms[term]; ok {
			matched++
		}
	}
	if matched == 0 {
		return 0, nil
	}

	denominator := math.Sqrt(float64(len(plan.CharTerms) * len(chunk.CharTerms)))
	if denominator <= 0 {
		return 0, nil
	}
	score := float64(matched) / denominator
	if score < 0.08 {
		return 0, nil
	}
	return score, []string{"semantic:char_overlap"}
}

func boostScore(plan queryPlan, chunk knowledgeApp.Chunk) (float64, []string) {
	if plan.Normalized == "" || chunk.Normalized == "" {
		return 0, nil
	}

	score := 0.0
	reasons := make([]string, 0, 3)
	titleNormalized := normalizeText(chunk.DocumentTitle)
	titleTerms := strings.Fields(titleNormalized)
	joinedTerms := strings.Join(plan.Terms, " ")
	if joinedTerms != "" && strings.Contains(chunk.Normalized, joinedTerms) {
		score += 0.65
		reasons = append(reasons, "exact_phrase")
	}
	if strings.Contains(chunk.Normalized, plan.Normalized) || strings.Contains(plan.Normalized, chunk.Normalized) {
		score += 0.4
		reasons = append(reasons, "normalized_substring")
	}
	if titleNormalized != "" && (strings.Contains(titleNormalized, plan.Normalized) || strings.Contains(plan.Normalized, titleNormalized)) {
		score += 0.45
		reasons = append(reasons, "title_hit")
	}
	if lexicalOverlap(plan.Expanded, titleTerms) >= 0.12 {
		score += 0.3
		reasons = append(reasons, "title_hit")
	}
	for _, term := range plan.Terms {
		if term != "" && term == strings.ToLower(chunk.Category) {
			score += 0.2
			reasons = append(reasons, "category_hit")
			break
		}
	}
	if score > 1 {
		score = 1
	}
	return score, reasons
}

func rerank(scored []scoredChunk, limit int) []Reference {
	if limit <= 0 {
		limit = 4
	}

	items := make([]Reference, 0, min(limit, len(scored)))
	docHits := make(map[string]int, len(scored))
	seenChunk := make(map[string]struct{}, len(scored))

	for _, candidate := range scored {
		key := candidate.Chunk.DocumentID + "|" + candidate.Chunk.Content
		if _, exists := seenChunk[key]; exists {
			continue
		}
		seenChunk[key] = struct{}{}

		score := candidate.Score
		if count := docHits[candidate.Chunk.DocumentID]; count > 0 {
			score = score * math.Pow(0.88, float64(count))
		}
		docHits[candidate.Chunk.DocumentID]++

		items = append(items, Reference{
			DocumentID:    candidate.Chunk.DocumentID,
			DocumentTitle: candidate.Chunk.DocumentTitle,
			Category:      candidate.Chunk.Category,
			ChunkIndex:    candidate.Chunk.Index,
			Chunk:         candidate.Chunk.Content,
			Score:         round(score),
			LexicalScore:  round(candidate.LexicalScore),
			SemanticScore: round(candidate.SemanticScore),
			BoostScore:    round(candidate.BoostScore),
			MatchReasons:  humanizeReasons(candidate.Reasons),
		})
		if len(items) >= limit {
			break
		}
	}
	return items
}

func charTerms(normalized string) []string {
	runes := []rune(strings.ReplaceAll(normalized, " ", ""))
	if len(runes) == 0 {
		return nil
	}

	items := make([]string, 0, len(runes)*2)
	seen := make(map[string]struct{}, len(runes)*2)
	for size := 2; size <= min(4, len(runes)); size++ {
		for start := 0; start+size <= len(runes); start++ {
			token := string(runes[start : start+size])
			if _, exists := seen[token]; exists {
				continue
			}
			seen[token] = struct{}{}
			items = append(items, token)
		}
	}
	return items
}

func humanizeReasons(reasons []string) []string {
	items := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		switch {
		case strings.HasPrefix(reason, "lexical:"):
			items = append(items, "关键词命中")
		case strings.HasPrefix(reason, "semantic:"):
			items = append(items, "语义命中")
		case reason == "exact_phrase":
			items = append(items, "完整短语命中")
		case reason == "normalized_substring":
			items = append(items, "上下文增强")
		case reason == "title_hit":
			items = append(items, "标题命中")
		case reason == "category_hit":
			items = append(items, "分类增强")
		default:
			items = append(items, reason)
		}
	}
	return uniqueNonEmpty(items)
}

func uniqueNonEmpty(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func lexicalOverlap(queryTerms []string, chunkTerms []string) float64 {
	if len(queryTerms) == 0 || len(chunkTerms) == 0 {
		return 0
	}

	chunkSet := make(map[string]struct{}, len(chunkTerms))
	for _, term := range chunkTerms {
		chunkSet[term] = struct{}{}
	}

	matched := 0
	for _, term := range queryTerms {
		if _, ok := chunkSet[term]; ok {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	return float64(matched) / float64(len(queryTerms))
}

func titleTerms(normalized string) []string {
	parts := strings.Fields(normalized)
	items := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		items = append(items, part)
		items = append(items, expandQueryToken(part)...)
	}
	return uniqueNonEmpty(items)
}

func round(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
