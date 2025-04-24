package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// SearchConfig 搜索配置
type SearchConfig struct {
	IndexPath     string   // 索引文件路径
	MaxResults    int      // 最大结果数
	MinScore      float64  // 最小相关度分数
	SearchFields  []string // 搜索字段
	FilterFields  []string // 过滤字段
	SortFields    []string // 排序字段
	HighlightTags [2]string // 高亮标签
}

// SearchResult 搜索结果
type SearchResult struct {
	ID          string                 `json:"id"`
	Score       float64                `json:"score"`
	Data        map[string]interface{} `json:"data"`
	Highlights  map[string][]string    `json:"highlights,omitempty"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Query       string                 // 搜索关键词
	Filters     map[string]interface{} // 过滤条件
	Sort        map[string]string      // 排序条件
	Page        int                    // 页码
	PageSize    int                    // 每页大小
	Fields      []string              // 返回字段
	MinScore    float64               // 最小相关度分数
}

// Search 搜索服务
type Search struct {
	config *SearchConfig
	logger *Logger
	mu     sync.RWMutex
	index  map[string]*Document
}

// Document 文档
type Document struct {
	ID        string                 `json:"id"`
	Data      map[string]interface{} `json:"data"`
	Keywords  map[string][]string    `json:"keywords"`
	UpdatedAt int64                  `json:"updated_at"`
}

// NewSearch 创建搜索服务实例
func NewSearch(config *SearchConfig, logger *Logger) *Search {
	return &Search{
		config: config,
		logger: logger,
		index:  make(map[string]*Document),
	}
}

// Index 索引文档
func (s *Search) Index(id string, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 提取关键词
	keywords := make(map[string][]string)
	for _, field := range s.config.SearchFields {
		if value, ok := data[field]; ok {
			if str, ok := value.(string); ok {
				keywords[field] = s.extractKeywords(str)
			}
		}
	}

	// 创建文档
	doc := &Document{
		ID:        id,
		Data:      data,
		Keywords:  keywords,
		UpdatedAt: time.Now().Unix(),
	}

	s.index[id] = doc
	return nil
}

// BatchIndex 批量索引文档
func (s *Search) BatchIndex(documents map[string]map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, data := range documents {
		// 提取关键词
		keywords := make(map[string][]string)
		for _, field := range s.config.SearchFields {
			if value, ok := data[field]; ok {
				if str, ok := value.(string); ok {
					keywords[field] = s.extractKeywords(str)
				}
			}
		}

		// 创建文档
		doc := &Document{
			ID:        id,
			Data:      data,
			Keywords:  keywords,
			UpdatedAt: time.Now().Unix(),
		}

		s.index[id] = doc
	}

	return nil
}

// Delete 删除文档
func (s *Search) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.index[id]; !exists {
		return fmt.Errorf("document not found: %s", id)
	}

	delete(s.index, id)
	return nil
}

// Search 执行搜索
func (s *Search) Search(query *SearchQuery) ([]*SearchResult, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 执行搜索
	results := make([]*SearchResult, 0)
	for _, doc := range s.index {
		// 计算相关度分数
		score := s.calculateScore(doc, query.Query)
		if score < query.MinScore {
			continue
		}

		// 应用过滤条件
		if !s.applyFilters(doc, query.Filters) {
			continue
		}

		// 创建搜索结果
		result := &SearchResult{
			ID:     doc.ID,
			Score:  score,
			Data:   s.filterFields(doc.Data, query.Fields),
		}

		// 添加高亮
		if query.Query != "" {
			result.Highlights = s.highlightKeywords(doc, query.Query)
		}

		results = append(results, result)
	}

	// 应用排序
	s.sortResults(results, query.Sort)

	// 应用分页
	total := len(results)
	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize
	if start >= total {
		return []*SearchResult{}, total, nil
	}
	if end > total {
		end = total
	}

	return results[start:end], total, nil
}

// extractKeywords 提取关键词
func (s *Search) extractKeywords(text string) []string {
	// 分词并转换为小写
	words := strings.Fields(strings.ToLower(text))
	
	// 去重
	seen := make(map[string]bool)
	keywords := make([]string, 0, len(words))
	for _, word := range words {
		if !seen[word] {
			seen[word] = true
			keywords = append(keywords, word)
		}
	}
	
	return keywords
}

// calculateScore 计算相关度分数
func (s *Search) calculateScore(doc *Document, query string) float64 {
	if query == "" {
		return 1.0
	}

	queryWords := s.extractKeywords(query)
	if len(queryWords) == 0 {
		return 0.0
	}

	var totalScore float64
	var totalWords int

	for field, keywords := range doc.Keywords {
		fieldScore := 0.0
		for _, queryWord := range queryWords {
			for _, keyword := range keywords {
				if strings.Contains(keyword, queryWord) {
					fieldScore += 1.0
					break
				}
			}
		}
		totalScore += fieldScore
		totalWords += len(keywords)
	}

	if totalWords == 0 {
		return 0.0
	}

	return totalScore / float64(totalWords)
}

// applyFilters 应用过滤条件
func (s *Search) applyFilters(doc *Document, filters map[string]interface{}) bool {
	for field, value := range filters {
		if docValue, ok := doc.Data[field]; !ok || docValue != value {
			return false
		}
	}
	return true
}

// filterFields 过滤字段
func (s *Search) filterFields(data map[string]interface{}, fields []string) map[string]interface{} {
	if len(fields) == 0 {
		return data
	}

	filtered := make(map[string]interface{})
	for _, field := range fields {
		if value, ok := data[field]; ok {
			filtered[field] = value
		}
	}
	return filtered
}

// sortResults 排序结果
func (s *Search) sortResults(results []*SearchResult, sort map[string]string) {
	if len(sort) == 0 {
		// 默认按相关度分数排序
		sort = map[string]string{"score": "desc"}
	}

	// 实现排序逻辑
	// TODO: 实现多字段排序
}

// highlightKeywords 高亮关键词
func (s *Search) highlightKeywords(doc *Document, query string) map[string][]string {
	highlights := make(map[string][]string)
	queryWords := s.extractKeywords(query)

	for field, keywords := range doc.Keywords {
		fieldHighlights := make([]string, 0)
		for _, keyword := range keywords {
			for _, queryWord := range queryWords {
				if strings.Contains(keyword, queryWord) {
					highlighted := strings.ReplaceAll(
						keyword,
						queryWord,
						fmt.Sprintf("%s%s%s",
							s.config.HighlightTags[0],
							queryWord,
							s.config.HighlightTags[1],
						),
					)
					fieldHighlights = append(fieldHighlights, highlighted)
					break
				}
			}
		}
		if len(fieldHighlights) > 0 {
			highlights[field] = fieldHighlights
		}
	}

	return highlights
}

// GetDocument 获取文档
func (s *Search) GetDocument(id string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, exists := s.index[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	return doc, nil
}

// UpdateDocument 更新文档
func (s *Search) UpdateDocument(id string, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, exists := s.index[id]
	if !exists {
		return fmt.Errorf("document not found: %s", id)
	}

	// 更新数据
	for k, v := range data {
		doc.Data[k] = v
	}

	// 更新关键词
	for _, field := range s.config.SearchFields {
		if value, ok := data[field]; ok {
			if str, ok := value.(string); ok {
				doc.Keywords[field] = s.extractKeywords(str)
			}
		}
	}

	doc.UpdatedAt = time.Now().Unix()
	return nil
}

// GetStats 获取索引统计信息
func (s *Search) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"total_documents": len(s.index),
		"index_size":      len(s.index),
		"search_fields":   s.config.SearchFields,
		"filter_fields":   s.config.FilterFields,
		"sort_fields":     s.config.SortFields,
	}
} 