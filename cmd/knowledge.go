// knowledge.go는 지식 허브 관련 CLI 명령어를 구현합니다.
// knowledge list/show/search/create/update/delete/upload/stats/folder 서브커맨드 제공
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// KnowledgeEntry는 지식 허브 항목을 나타냅니다.
type KnowledgeEntry struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content,omitempty"`
	Category   string   `json:"category,omitempty"`
	SourceType string   `json:"source_type,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Importance *int     `json:"importance,omitempty"`
	CreatedAt  string   `json:"created_at,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
}

// KnowledgeSearchResult는 지식 검색 결과 항목을 나타냅니다.
type KnowledgeSearchResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Score    float64 `json:"score,omitempty"`
	Category string  `json:"category,omitempty"`
	Snippet  string  `json:"snippet,omitempty"`
}

// KnowledgeStats는 지식 허브 통계를 나타냅니다.
type KnowledgeStats struct {
	TotalEntries      int            `json:"total_entries"`
	ByCategory        map[string]int `json:"by_category,omitempty"`
	BySourceType      map[string]int `json:"by_source_type,omitempty"`
	AverageImportance float64        `json:"average_importance,omitempty"`
}

// KnowledgeFolder는 지식 폴더를 나타냅니다.
type KnowledgeFolder struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	FileCount int    `json:"file_count,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// KnowledgeFolderFile는 지식 폴더 내 파일을 나타냅니다.
type KnowledgeFolderFile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
	Size   int64  `json:"size,omitempty"`
}

// FolderSyncResult는 폴더 동기화 결과를 나타냅니다.
type FolderSyncResult struct {
	FolderID     string `json:"folder_id"`
	Status       string `json:"status"`
	FilesAdded   int    `json:"files_added,omitempty"`
	FilesUpdated int    `json:"files_updated,omitempty"`
	FilesRemoved int    `json:"files_removed,omitempty"`
}

// FolderBrowseEntry는 디렉토리 탐색 결과 항목을 나타냅니다.
type FolderBrowseEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
	Path string `json:"path,omitempty"`
}

// knowledgeFoldersResponse는 폴더 목록 응답의 래퍼입니다.
type knowledgeFoldersResponse struct {
	Folders []KnowledgeFolder `json:"folders"`
}

// knowledgeBrowseResponse는 디렉토리 탐색 응답의 래퍼입니다.
type knowledgeBrowseResponse struct {
	Entries []FolderBrowseEntry `json:"entries"`
}

var (
	knowledgeListCategory      string
	knowledgeListSourceType    string
	knowledgeListTags          []string
	knowledgeListImportanceMin int
	knowledgeListPage          int
	knowledgeListPerPage       int
	knowledgeSearchLimit       int
	knowledgeSearchCategory    string
	knowledgeSearchTags        []string
	knowledgeCreateTitle       string
	knowledgeCreateContent     string
	knowledgeCreateCategory    string
	knowledgeCreateSourceType  string
	knowledgeCreateTags        []string
	knowledgeCreateImportance  int
	knowledgeUploadCategory    string
)

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge",
	Short: "지식 허브 관리",
	Long:  "지식 항목 목록 조회, 검색, 생성, 수정, 삭제 및 파일 업로드 기능을 제공합니다.",
}

var knowledgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "지식 항목 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		importance, _ := cmd.Flags().GetInt("importance")
		tags, _ := cmd.Flags().GetStringArray("tags")
		return runKnowledgeList(client, os.Stdout, knowledgeListCategory, knowledgeListSourceType, tags, importance, page, perPage, jsonOut)
	},
}

var knowledgeShowCmd = &cobra.Command{
	Use:  "show <entry-id>",
	Short: "지식 항목 상세 조회",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeShow(client, os.Stdout, args[0], jsonOut)
	},
}

var knowledgeSearchCmd = &cobra.Command{
	Use:  "search <query>",
	Short: "지식 항목 검색",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		limit, _ := cmd.Flags().GetInt("limit")
		tags, _ := cmd.Flags().GetStringArray("tags")
		return runKnowledgeSearch(client, os.Stdout, args[0], limit, knowledgeSearchCategory, tags, jsonOut)
	},
}

var knowledgeCreateCmd = &cobra.Command{
	Use:  "create",
	Short: "지식 항목 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		importance, _ := cmd.Flags().GetInt("importance")
		tags, _ := cmd.Flags().GetStringArray("tags")
		return runKnowledgeCreate(client, os.Stdout, knowledgeCreateTitle, knowledgeCreateContent, knowledgeCreateCategory, knowledgeCreateSourceType, tags, importance, jsonOut)
	},
}

var knowledgeUpdateCmd = &cobra.Command{
	Use:  "update <entry-id>",
	Short: "지식 항목 수정",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		tags, _ := cmd.Flags().GetStringArray("tags")
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		category, _ := cmd.Flags().GetString("category")
		return runKnowledgeUpdate(client, os.Stdout, args[0], title, content, category, tags, jsonOut)
	},
}

var knowledgeDeleteCmd = &cobra.Command{
	Use:  "delete <entry-id>",
	Short: "지식 항목 삭제",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runKnowledgeDelete(client, os.Stdout, args[0])
	},
}

var knowledgeUploadCmd = &cobra.Command{
	Use:  "upload <file-path>",
	Short: "파일을 지식 허브에 업로드",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeUpload(client, os.Stdout, args[0], knowledgeUploadCategory, jsonOut)
	},
}

var knowledgeStatsCmd = &cobra.Command{
	Use:  "stats",
	Short: "지식 허브 통계 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeStats(client, os.Stdout, jsonOut)
	},
}

var knowledgeFolderCmd = &cobra.Command{
	Use:  "folder",
	Short: "지식 폴더 관리",
}

var knowledgeFolderListCmd = &cobra.Command{
	Use:  "list",
	Short: "폴더 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeFolderList(client, os.Stdout, jsonOut)
	},
}

var knowledgeFolderShowCmd = &cobra.Command{
	Use:  "show <folder-id>",
	Short: "폴더 상세 조회",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeFolderShow(client, os.Stdout, args[0], jsonOut)
	},
}

var knowledgeFolderCreateCmd = &cobra.Command{
	Use:  "create <path>",
	Short: "폴더 생성",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeFolderCreate(client, os.Stdout, args[0], jsonOut)
	},
}

var knowledgeFolderSyncCmd = &cobra.Command{
	Use:  "sync <folder-id>",
	Short: "폴더 동기화",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeFolderSync(client, os.Stdout, args[0], jsonOut)
	},
}

var knowledgeFolderFilesCmd = &cobra.Command{
	Use:  "files <folder-id>",
	Short: "폴더 파일 목록 조회",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		status, _ := cmd.Flags().GetString("status")
		name, _ := cmd.Flags().GetString("name")
		return runKnowledgeFolderFiles(client, os.Stdout, args[0], status, name, page, perPage, jsonOut)
	},
}

var knowledgeFolderBrowseCmd = &cobra.Command{
	Use:  "browse <path>",
	Short: "디렉토리 탐색",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runKnowledgeFolderBrowse(client, os.Stdout, args[0], jsonOut)
	},
}

var knowledgeFolderDeleteCmd = &cobra.Command{
	Use:  "delete <folder-id>",
	Short: "폴더 삭제",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runKnowledgeFolderDelete(client, os.Stdout, args[0])
	},
}

func init() {
	rootCmd.AddCommand(knowledgeCmd)
	knowledgeCmd.AddCommand(knowledgeListCmd, knowledgeShowCmd, knowledgeSearchCmd)
	knowledgeCmd.AddCommand(knowledgeCreateCmd, knowledgeUpdateCmd, knowledgeDeleteCmd)
	knowledgeCmd.AddCommand(knowledgeUploadCmd, knowledgeStatsCmd, knowledgeFolderCmd)
	knowledgeCmd.AddCommand(knowledgeSyncCmd)

	knowledgeFolderCmd.AddCommand(knowledgeFolderListCmd, knowledgeFolderShowCmd)
	knowledgeFolderCmd.AddCommand(knowledgeFolderCreateCmd, knowledgeFolderSyncCmd)
	knowledgeFolderCmd.AddCommand(knowledgeFolderFilesCmd, knowledgeFolderBrowseCmd)
	knowledgeFolderCmd.AddCommand(knowledgeFolderDeleteCmd)

	knowledgeListCmd.Flags().StringVar(&knowledgeListCategory, "category", "", "카테고리 필터")
	knowledgeListCmd.Flags().StringVar(&knowledgeListSourceType, "source-type", "", "소스 타입 필터")
	knowledgeListCmd.Flags().StringArrayVar(&knowledgeListTags, "tags", nil, "태그 필터")
	knowledgeListCmd.Flags().IntVar(&knowledgeListImportanceMin, "importance", 0, "최소 중요도 필터")
	knowledgeListCmd.Flags().IntVar(&knowledgeListPage, "page", 0, "페이지 번호")
	knowledgeListCmd.Flags().IntVar(&knowledgeListPerPage, "per-page", 0, "페이지당 항목 수")

	knowledgeSearchCmd.Flags().IntVar(&knowledgeSearchLimit, "limit", 10, "검색 결과 최대 수")
	knowledgeSearchCmd.Flags().StringVar(&knowledgeSearchCategory, "category", "", "카테고리 필터")
	knowledgeSearchCmd.Flags().StringArrayVar(&knowledgeSearchTags, "tags", nil, "태그 필터")

	knowledgeCreateCmd.Flags().StringVar(&knowledgeCreateTitle, "title", "", "항목 제목")
	knowledgeCreateCmd.Flags().StringVar(&knowledgeCreateContent, "content", "", "항목 내용")
	knowledgeCreateCmd.Flags().StringVar(&knowledgeCreateCategory, "category", "", "카테고리")
	knowledgeCreateCmd.Flags().StringVar(&knowledgeCreateSourceType, "source-type", "", "소스 타입")
	knowledgeCreateCmd.Flags().StringArrayVar(&knowledgeCreateTags, "tags", nil, "태그")
	knowledgeCreateCmd.Flags().IntVar(&knowledgeCreateImportance, "importance", 0, "중요도 (1-10)")

	knowledgeUpdateCmd.Flags().String("title", "", "수정할 제목")
	knowledgeUpdateCmd.Flags().String("content", "", "수정할 내용")
	knowledgeUpdateCmd.Flags().String("category", "", "수정할 카테고리")
	knowledgeUpdateCmd.Flags().StringArray("tags", nil, "수정할 태그")

	knowledgeUploadCmd.Flags().StringVar(&knowledgeUploadCategory, "category", "", "업로드 카테고리")

	for _, sub := range []*cobra.Command{
		knowledgeListCmd, knowledgeShowCmd, knowledgeSearchCmd, knowledgeCreateCmd,
		knowledgeUpdateCmd, knowledgeStatsCmd, knowledgeUploadCmd,
		knowledgeFolderListCmd, knowledgeFolderShowCmd, knowledgeFolderCreateCmd,
		knowledgeFolderSyncCmd, knowledgeFolderFilesCmd, knowledgeFolderBrowseCmd,
	} {
		sub.Flags().Bool("json", false, "JSON 형식으로 출력")
	}

	knowledgeFolderFilesCmd.Flags().Int("page", 0, "페이지 번호")
	knowledgeFolderFilesCmd.Flags().Int("per-page", 0, "페이지당 항목 수")
	knowledgeFolderFilesCmd.Flags().String("status", "", "상태 필터")
	knowledgeFolderFilesCmd.Flags().String("name", "", "파일 이름 필터")
}

// runKnowledgeList는 지식 항목 목록을 조회하여 출력합니다.
func runKnowledgeList(client *apiclient.Client, out io.Writer, category, sourceType string, tags []string, importance, page, perPage int, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	params := url.Values{}
	if category != "" {
		params.Set("category", category)
	}
	if sourceType != "" {
		params.Set("source_type", sourceType)
	}
	for _, tag := range tags {
		params.Add("tags", tag)
	}
	if importance > 0 {
		params.Set("importance", strconv.Itoa(importance))
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		params.Set("per_page", strconv.Itoa(perPage))
	}

	path := "/api/v1/workspaces/" + workspaceID + "/knowledge"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	entries, _, err := apiclient.DoPage[KnowledgeEntry](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("지식 항목 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, entries)
	}

	headers := []string{"ID", "TITLE", "CATEGORY", "SOURCE_TYPE", "IMPORTANCE"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		imp := ""
		if e.Importance != nil {
			imp = strconv.Itoa(*e.Importance)
		}
		rows[i] = []string{e.ID, e.Title, e.Category, e.SourceType, imp}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runKnowledgeShow는 지식 항목 상세를 조회하여 출력합니다.
func runKnowledgeShow(client *apiclient.Client, out io.Writer, entryID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(entryID); err != nil {
		return err
	}
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	entry, err := apiclient.Do[KnowledgeEntry](client, ctx, "GET", "/api/v1/knowledge/"+entryID, nil)
	if err != nil {
		return fmt.Errorf("지식 항목 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, entry)
	}

	imp := ""
	if entry.Importance != nil {
		imp = strconv.Itoa(*entry.Importance)
	}
	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: entry.ID},
		{Key: "Title", Value: entry.Title},
		{Key: "Category", Value: entry.Category},
		{Key: "SourceType", Value: entry.SourceType},
		{Key: "Importance", Value: imp},
		{Key: "Tags", Value: strings.Join(entry.Tags, ", ")},
		{Key: "Content", Value: entry.Content},
		{Key: "CreatedAt", Value: entry.CreatedAt},
	})
	return nil
}

// runKnowledgeSearch는 지식 항목을 검색합니다.
func runKnowledgeSearch(client *apiclient.Client, out io.Writer, query string, limit int, category string, tags []string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	reqBody := map[string]interface{}{"query": query, "limit": limit}
	if category != "" {
		reqBody["category"] = category
	}
	if len(tags) > 0 {
		reqBody["tags"] = tags
	}

	results, err := apiclient.DoList[KnowledgeSearchResult](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/knowledge/search", reqBody)
	if err != nil {
		return fmt.Errorf("지식 검색 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, results)
	}

	headers := []string{"ID", "TITLE", "SCORE", "CATEGORY"}
	rows := make([][]string, len(results))
	for i, r := range results {
		rows[i] = []string{r.ID, r.Title, fmt.Sprintf("%.2f", r.Score), r.Category}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runKnowledgeCreate는 새 지식 항목을 생성합니다.
func runKnowledgeCreate(client *apiclient.Client, out io.Writer, title, content, category, sourceType string, tags []string, importance int, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	reqBody := map[string]interface{}{"title": title}
	if content != "" {
		reqBody["content"] = content
	}
	if category != "" {
		reqBody["category"] = category
	}
	if sourceType != "" {
		reqBody["source_type"] = sourceType
	}
	if len(tags) > 0 {
		reqBody["tags"] = tags
	}
	if importance > 0 {
		reqBody["importance"] = importance
	}

	entry, err := apiclient.Do[KnowledgeEntry](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/knowledge", reqBody)
	if err != nil {
		return fmt.Errorf("지식 항목 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, entry)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: entry.ID},
		{Key: "Title", Value: entry.Title},
		{Key: "Category", Value: entry.Category},
	})
	return nil
}

// runKnowledgeUpdate는 지식 항목을 수정합니다.
func runKnowledgeUpdate(client *apiclient.Client, out io.Writer, entryID, title, content, category string, tags []string, jsonOutput bool) error {
	if err := apiclient.ValidateID(entryID); err != nil {
		return err
	}
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	reqBody := map[string]interface{}{}
	if title != "" {
		reqBody["title"] = title
	}
	if content != "" {
		reqBody["content"] = content
	}
	if category != "" {
		reqBody["category"] = category
	}
	if len(tags) > 0 {
		reqBody["tags"] = tags
	}

	entry, err := apiclient.Do[KnowledgeEntry](client, ctx, "PATCH",
		"/api/v1/knowledge/"+entryID, reqBody)
	if err != nil {
		return fmt.Errorf("지식 항목 수정 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, entry)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: entry.ID},
		{Key: "Title", Value: entry.Title},
	})
	return nil
}

// runKnowledgeDelete는 지식 항목을 삭제합니다.
func runKnowledgeDelete(client *apiclient.Client, out io.Writer, entryID string) error {
	if err := apiclient.ValidateID(entryID); err != nil {
		return err
	}
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := client.Delete(ctx, "/api/v1/knowledge/"+entryID)
	if err != nil {
		return fmt.Errorf("지식 항목 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "지식 항목 %s 삭제 완료\n", entryID)
	return nil
}

// runKnowledgeUpload는 파일을 지식 허브에 업로드합니다.
func runKnowledgeUpload(client *apiclient.Client, out io.Writer, filePath, category string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(30 * time.Second)
	defer cancel()

	workspaceID := client.WorkspaceID()
	extraFields := map[string]string{}
	if category != "" {
		extraFields["category"] = category
	} else {
		extraFields["category"] = "operations"
	}

	rawData, err := apiclient.DoUpload(client, ctx,
		"/api/v1/workspaces/"+workspaceID+"/knowledge/upload",
		filePath, extraFields)
	if err != nil {
		return fmt.Errorf("파일 업로드 실패: %w", err)
	}

	var entry KnowledgeEntry
	if err := json.Unmarshal(*rawData, &entry); err != nil {
		return fmt.Errorf("업로드 응답 파싱 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, entry)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: entry.ID},
		{Key: "Title", Value: entry.Title},
	})
	return nil
}

// runKnowledgeStats는 지식 허브 통계를 조회합니다.
func runKnowledgeStats(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	stats, err := apiclient.Do[KnowledgeStats](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/knowledge/stats", nil)
	if err != nil {
		return fmt.Errorf("지식 허브 통계 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, stats)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "TotalEntries", Value: strconv.Itoa(stats.TotalEntries)},
		{Key: "AvgImportance", Value: fmt.Sprintf("%.1f", stats.AverageImportance)},
	})
	return nil
}

// runKnowledgeFolderList는 폴더 목록을 조회합니다.
func runKnowledgeFolderList(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	resp, err := apiclient.Do[knowledgeFoldersResponse](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/knowledge/folders", nil)
	if err != nil {
		return fmt.Errorf("폴더 목록 조회 실패: %w", err)
	}

	folders := resp.Folders
	if jsonOutput {
		return apiclient.PrintJSON(out, folders)
	}

	headers := []string{"ID", "PATH", "NAME", "STATUS", "FILE_COUNT"}
	rows := make([][]string, len(folders))
	for i, f := range folders {
		rows[i] = []string{f.ID, f.Path, f.Name, f.Status, strconv.Itoa(f.FileCount)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runKnowledgeFolderShow는 폴더 상세를 조회합니다.
func runKnowledgeFolderShow(client *apiclient.Client, out io.Writer, folderID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(folderID); err != nil {
		return err
	}
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	folder, err := apiclient.Do[KnowledgeFolder](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/knowledge/folders/"+folderID, nil)
	if err != nil {
		return fmt.Errorf("폴더 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, folder)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: folder.ID},
		{Key: "Path", Value: folder.Path},
		{Key: "Name", Value: folder.Name},
		{Key: "Status", Value: folder.Status},
		{Key: "FileCount", Value: strconv.Itoa(folder.FileCount)},
		{Key: "CreatedAt", Value: folder.CreatedAt},
	})
	return nil
}

// runKnowledgeFolderCreate는 새 폴더를 생성합니다.
func runKnowledgeFolderCreate(client *apiclient.Client, out io.Writer, folderPath string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	folder, err := apiclient.Do[KnowledgeFolder](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/knowledge/folders",
		map[string]interface{}{"path": folderPath})
	if err != nil {
		return fmt.Errorf("폴더 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, folder)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: folder.ID},
		{Key: "Path", Value: folder.Path},
		{Key: "Status", Value: folder.Status},
	})
	return nil
}

// runKnowledgeFolderSync는 폴더를 동기화합니다.
func runKnowledgeFolderSync(client *apiclient.Client, out io.Writer, folderID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(folderID); err != nil {
		return err
	}
	ctx, cancel := apiclient.NewContextWithTimeout(30 * time.Second)
	defer cancel()

	workspaceID := client.WorkspaceID()
	result, err := apiclient.Do[FolderSyncResult](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/knowledge/folders/"+folderID+"/sync", nil)
	if err != nil {
		return fmt.Errorf("폴더 동기화 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, result)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "FolderID", Value: result.FolderID},
		{Key: "Status", Value: result.Status},
		{Key: "FilesAdded", Value: strconv.Itoa(result.FilesAdded)},
		{Key: "FilesUpdated", Value: strconv.Itoa(result.FilesUpdated)},
	})
	return nil
}

// runKnowledgeFolderFiles는 폴더 파일 목록을 조회합니다.
func runKnowledgeFolderFiles(client *apiclient.Client, out io.Writer, folderID, statusFilter, nameFilter string, page, perPage int, jsonOutput bool) error {
	if err := apiclient.ValidateID(folderID); err != nil {
		return err
	}
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	params := url.Values{}
	if statusFilter != "" {
		params.Set("status", statusFilter)
	}
	if nameFilter != "" {
		params.Set("search", nameFilter)
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		params.Set("per_page", strconv.Itoa(perPage))
	}

	apiPath := "/api/v1/workspaces/" + workspaceID + "/knowledge/folders/" + folderID + "/files"
	if len(params) > 0 {
		apiPath += "?" + params.Encode()
	}

	files, _, err := apiclient.DoPage[KnowledgeFolderFile](client, ctx, "GET", apiPath, nil)
	if err != nil {
		return fmt.Errorf("폴더 파일 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, files)
	}

	headers := []string{"ID", "NAME", "STATUS", "SIZE"}
	rows := make([][]string, len(files))
	for i, f := range files {
		rows[i] = []string{f.ID, f.Name, f.Status, strconv.FormatInt(f.Size, 10)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runKnowledgeFolderBrowse는 디렉토리를 탐색합니다.
func runKnowledgeFolderBrowse(client *apiclient.Client, out io.Writer, browsePath string, jsonOutput bool) error {
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	resp, err := apiclient.Do[knowledgeBrowseResponse](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/knowledge/folders/browse",
		map[string]interface{}{"path": browsePath})
	if err != nil {
		return fmt.Errorf("디렉토리 탐색 실패: %w", err)
	}

	entries := resp.Entries
	if jsonOutput {
		return apiclient.PrintJSON(out, entries)
	}

	headers := []string{"NAME", "TYPE", "SIZE"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{e.Name, e.Type, strconv.FormatInt(e.Size, 10)}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runKnowledgeFolderDelete는 폴더를 삭제합니다.
func runKnowledgeFolderDelete(client *apiclient.Client, out io.Writer, folderID string) error {
	if err := apiclient.ValidateID(folderID); err != nil {
		return err
	}
	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	workspaceID := client.WorkspaceID()
	_, err := client.Delete(ctx, "/api/v1/workspaces/"+workspaceID+"/knowledge/folders/"+folderID)
	if err != nil {
		return fmt.Errorf("폴더 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "폴더 %s 삭제 완료\n", folderID)
	return nil
}
