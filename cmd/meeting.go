// meeting.go는 미팅 관련 CLI 명령어를 구현합니다.
// meeting list/show/create/start/end/cancel/messages/regenerate-minutes/schedule-create/schedules 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Meeting은 미팅 기본 정보를 나타냅니다.
type Meeting struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Agenda       string   `json:"agenda,omitempty"`
	Status       string   `json:"status,omitempty"`
	ChannelID    string   `json:"channel_id,omitempty"`
	Participants []string `json:"participants,omitempty"`
	ScheduledAt  string   `json:"scheduled_at,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	EndedAt      string   `json:"ended_at,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
}

// MeetingMessage는 미팅 메시지를 나타냅니다.
type MeetingMessage struct {
	ID        string `json:"id"`
	MeetingID string `json:"meeting_id"`
	Sender    string `json:"sender,omitempty"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at,omitempty"`
}

// MeetingSchedule은 반복 미팅 스케줄을 나타냅니다.
type MeetingSchedule struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	CronExpression string   `json:"cron_expression,omitempty"`
	Participants   []string `json:"participant_agent_ids,omitempty"`
	RecurrenceType string   `json:"recurrence_type,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
}

var (
	meetingJSONOutput         bool
	meetingListStatus         string
	meetingListPage           int
	meetingListPerPage        int
	meetingTitle              string
	meetingAgenda             string
	meetingChannelID          string
	meetingParticipants       string
	meetingScheduledAt        string
	meetingScheduleTitle      string
	meetingScheduleAgenda     string
	meetingScheduleCron       string
	meetingScheduleParticipants string
	meetingScheduleRecurrence string
)

// meetingCmd는 meeting 서브커맨드의 루트입니다.
var meetingCmd = &cobra.Command{
	Use:   "meeting",
	Short: "미팅 관련 명령어",
	Long:  `미팅 목록 조회, 상세 조회, 생성, 시작, 종료, 취소, 메시지 조회, 회의록 재생성, 스케줄 관리 기능을 제공합니다.`,
}

var meetingListCmd = &cobra.Command{
	Use:   "list",
	Short: "미팅 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		status, _ := cmd.Flags().GetString("status")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		client.SetJSONOutput(jsonOut)
		return runMeetingList(client, os.Stdout, status, page, perPage, jsonOut)
	},
}

var meetingShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "미팅 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runMeetingShow(client, os.Stdout, args[0], jsonOut)
	},
}

var meetingCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "미팅 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		title, _ := cmd.Flags().GetString("title")
		agenda, _ := cmd.Flags().GetString("agenda")
		channelID, _ := cmd.Flags().GetString("channel-id")
		participants, _ := cmd.Flags().GetString("participants")
		scheduledAt, _ := cmd.Flags().GetString("scheduled-at")
		client.SetJSONOutput(jsonOut)
		return runMeetingCreate(client, os.Stdout, title, agenda, channelID, participants, scheduledAt, jsonOut)
	},
}

var meetingStartCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "미팅 시작",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runMeetingStart(client, os.Stdout, args[0])
	},
}

var meetingEndCmd = &cobra.Command{
	Use:   "end <id>",
	Short: "미팅 종료",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runMeetingEnd(client, os.Stdout, args[0])
	},
}

var meetingCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "미팅 취소",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runMeetingCancel(client, os.Stdout, args[0])
	},
}

var meetingMessagesCmd = &cobra.Command{
	Use:   "messages <id>",
	Short: "미팅 메시지 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runMeetingMessages(client, os.Stdout, args[0], jsonOut)
	},
}

var meetingRegenerateMinutesCmd = &cobra.Command{
	Use:   "regenerate-minutes <id>",
	Short: "회의록 재생성",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runMeetingRegenerateMinutes(client, os.Stdout, args[0])
	},
}

var meetingScheduleCreateCmd = &cobra.Command{
	Use:   "schedule-create",
	Short: "반복 미팅 스케줄 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		title, _ := cmd.Flags().GetString("title")
		agenda, _ := cmd.Flags().GetString("agenda")
		cronExpr, _ := cmd.Flags().GetString("cron")
		parts, _ := cmd.Flags().GetString("participants")
		recurrence, _ := cmd.Flags().GetString("recurrence")
		client.SetJSONOutput(jsonOut)
		return runMeetingScheduleCreate(client, os.Stdout, title, agenda, cronExpr, parts, recurrence, jsonOut)
	},
}

var meetingSchedulesCmd = &cobra.Command{
	Use:   "schedules",
	Short: "반복 미팅 스케줄 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(jsonOut)
		return runMeetingSchedules(client, os.Stdout, jsonOut)
	},
}

func init() {
	rootCmd.AddCommand(meetingCmd)
	meetingCmd.AddCommand(meetingListCmd)
	meetingCmd.AddCommand(meetingShowCmd)
	meetingCmd.AddCommand(meetingCreateCmd)
	meetingCmd.AddCommand(meetingStartCmd)
	meetingCmd.AddCommand(meetingEndCmd)
	meetingCmd.AddCommand(meetingCancelCmd)
	meetingCmd.AddCommand(meetingMessagesCmd)
	meetingCmd.AddCommand(meetingRegenerateMinutesCmd)
	meetingCmd.AddCommand(meetingScheduleCreateCmd)
	meetingCmd.AddCommand(meetingSchedulesCmd)

	// --json 플래그
	for _, sub := range []*cobra.Command{
		meetingListCmd, meetingShowCmd, meetingCreateCmd,
		meetingMessagesCmd, meetingScheduleCreateCmd, meetingSchedulesCmd,
	} {
		sub.Flags().BoolVar(&meetingJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 플래그
	meetingListCmd.Flags().StringVar(&meetingListStatus, "status", "", "상태 필터 (scheduled|in_progress|ended|cancelled)")
	meetingListCmd.Flags().IntVar(&meetingListPage, "page", 0, "페이지 번호")
	meetingListCmd.Flags().IntVar(&meetingListPerPage, "per-page", 0, "페이지당 항목 수")

	// create 전용 플래그
	meetingCreateCmd.Flags().StringVar(&meetingTitle, "title", "", "미팅 제목 (필수)")
	meetingCreateCmd.Flags().StringVar(&meetingAgenda, "agenda", "", "미팅 안건")
	meetingCreateCmd.Flags().StringVar(&meetingChannelID, "channel-id", "", "채널 ID")
	meetingCreateCmd.Flags().StringVar(&meetingParticipants, "participants", "", "참가자 ID (쉼표 구분)")
	meetingCreateCmd.Flags().StringVar(&meetingScheduledAt, "scheduled-at", "", "예약 시각 (RFC3339)")

	// schedule-create 전용 플래그
	meetingScheduleCreateCmd.Flags().StringVar(&meetingScheduleTitle, "title", "", "스케줄 제목 (필수)")
	meetingScheduleCreateCmd.Flags().StringVar(&meetingScheduleAgenda, "agenda", "", "스케줄 안건")
	meetingScheduleCreateCmd.Flags().StringVar(&meetingScheduleCron, "cron", "", "Cron 표현식")
	meetingScheduleCreateCmd.Flags().StringVar(&meetingScheduleParticipants, "participants", "", "참가자 에이전트 ID (쉼표 구분)")
	meetingScheduleCreateCmd.Flags().StringVar(&meetingScheduleRecurrence, "recurrence", "", "반복 유형 (daily|weekly|monthly)")
}

// runMeetingList는 미팅 목록을 조회하고 출력합니다.
func runMeetingList(client *apiclient.Client, out io.Writer, status string, page, perPage int, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	path := "/api/v1/workspaces/" + workspaceID + "/meetings"

	// 쿼리 파라미터 구성
	params := []string{}
	if status != "" {
		params = append(params, "status="+status)
	}
	if page > 0 {
		params = append(params, fmt.Sprintf("page=%d", page))
	}
	if perPage > 0 {
		params = append(params, fmt.Sprintf("per_page=%d", perPage))
	}
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	meetings, err := apiclient.DoList[Meeting](client, ctx, "GET", path, nil)
	if err != nil {
		return fmt.Errorf("미팅 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, meetings)
	}

	headers := []string{"ID", "TITLE", "STATUS", "SCHEDULED_AT"}
	rows := make([][]string, len(meetings))
	for i, m := range meetings {
		shortID := m.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, m.Title, m.Status, m.ScheduledAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runMeetingShow는 미팅 상세 정보를 출력합니다.
func runMeetingShow(client *apiclient.Client, out io.Writer, meetingID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(meetingID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	meeting, err := apiclient.Do[Meeting](client, ctx, "GET", "/api/v1/meetings/"+meetingID, nil)
	if err != nil {
		return fmt.Errorf("미팅 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, meeting)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: meeting.ID},
		{Key: "Title", Value: meeting.Title},
		{Key: "Agenda", Value: meeting.Agenda},
		{Key: "Status", Value: meeting.Status},
		{Key: "ChannelID", Value: meeting.ChannelID},
		{Key: "ScheduledAt", Value: meeting.ScheduledAt},
		{Key: "StartedAt", Value: meeting.StartedAt},
		{Key: "EndedAt", Value: meeting.EndedAt},
		{Key: "CreatedAt", Value: meeting.CreatedAt},
	})
	return nil
}

// runMeetingCreate는 새 미팅을 생성합니다.
func runMeetingCreate(client *apiclient.Client, out io.Writer, title, agenda, channelID, participants, scheduledAt string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	body := map[string]interface{}{
		"title": title,
	}
	if agenda != "" {
		body["agenda"] = agenda
	}
	if channelID != "" {
		body["channel_id"] = channelID
	}
	if participants != "" {
		body["participants"] = strings.Split(participants, ",")
	}
	if scheduledAt != "" {
		body["scheduled_at"] = scheduledAt
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	meeting, err := apiclient.Do[Meeting](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/meetings", body)
	if err != nil {
		return fmt.Errorf("미팅 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, meeting)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: meeting.ID},
		{Key: "Title", Value: meeting.Title},
		{Key: "Status", Value: meeting.Status},
		{Key: "ScheduledAt", Value: meeting.ScheduledAt},
	})
	return nil
}

// runMeetingStart는 미팅을 시작합니다.
func runMeetingStart(client *apiclient.Client, out io.Writer, meetingID string) error {
	if err := apiclient.ValidateID(meetingID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	meeting, err := apiclient.Do[Meeting](client, ctx, "PATCH", "/api/v1/meetings/"+meetingID+"/start", nil)
	if err != nil {
		return fmt.Errorf("미팅 시작 실패: %w", err)
	}

	fmt.Fprintf(out, "미팅 시작: %s (%s)\n", meeting.Title, meeting.Status)
	return nil
}

// runMeetingEnd는 미팅을 종료합니다.
func runMeetingEnd(client *apiclient.Client, out io.Writer, meetingID string) error {
	if err := apiclient.ValidateID(meetingID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	meeting, err := apiclient.Do[Meeting](client, ctx, "PATCH", "/api/v1/meetings/"+meetingID+"/end", nil)
	if err != nil {
		return fmt.Errorf("미팅 종료 실패: %w", err)
	}

	fmt.Fprintf(out, "미팅 종료: %s (%s)\n", meeting.Title, meeting.Status)
	return nil
}

// runMeetingCancel은 미팅을 취소합니다.
func runMeetingCancel(client *apiclient.Client, out io.Writer, meetingID string) error {
	if err := apiclient.ValidateID(meetingID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	meeting, err := apiclient.Do[Meeting](client, ctx, "PATCH", "/api/v1/meetings/"+meetingID+"/cancel", nil)
	if err != nil {
		return fmt.Errorf("미팅 취소 실패: %w", err)
	}

	fmt.Fprintf(out, "미팅 취소: %s (%s)\n", meeting.Title, meeting.Status)
	return nil
}

// runMeetingMessages는 미팅 메시지 목록을 출력합니다.
func runMeetingMessages(client *apiclient.Client, out io.Writer, meetingID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(meetingID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	messages, err := apiclient.DoList[MeetingMessage](client, ctx, "GET",
		"/api/v1/meetings/"+meetingID+"/messages", nil)
	if err != nil {
		return fmt.Errorf("미팅 메시지 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, messages)
	}

	headers := []string{"ID", "SENDER", "CONTENT", "CREATED_AT"}
	rows := make([][]string, len(messages))
	for i, m := range messages {
		shortID := m.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, m.Sender, m.Content, m.CreatedAt}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runMeetingRegenerateMinutes는 회의록을 재생성합니다.
func runMeetingRegenerateMinutes(client *apiclient.Client, out io.Writer, meetingID string) error {
	if err := apiclient.ValidateID(meetingID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	meeting, err := apiclient.Do[Meeting](client, ctx, "POST",
		"/api/v1/meetings/"+meetingID+"/regenerate-minutes", nil)
	if err != nil {
		return fmt.Errorf("회의록 재생성 실패: %w", err)
	}

	fmt.Fprintf(out, "회의록 재생성 완료: %s\n", meeting.Title)
	return nil
}

// runMeetingScheduleCreate는 반복 미팅 스케줄을 생성합니다.
func runMeetingScheduleCreate(client *apiclient.Client, out io.Writer, title, agenda, cron, participants, recurrence string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	body := map[string]interface{}{
		"title": title,
	}
	if agenda != "" {
		body["agenda"] = agenda
	}
	if cron != "" {
		body["cron_expression"] = cron
	}
	if participants != "" {
		body["participant_agent_ids"] = strings.Split(participants, ",")
	}
	if recurrence != "" {
		body["recurrence_type"] = recurrence
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	schedule, err := apiclient.Do[MeetingSchedule](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/meeting-schedules", body)
	if err != nil {
		return fmt.Errorf("미팅 스케줄 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, schedule)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: schedule.ID},
		{Key: "Title", Value: schedule.Title},
		{Key: "Cron", Value: schedule.CronExpression},
		{Key: "Recurrence", Value: schedule.RecurrenceType},
	})
	return nil
}

// runMeetingSchedules는 반복 미팅 스케줄 목록을 출력합니다.
func runMeetingSchedules(client *apiclient.Client, out io.Writer, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	schedules, err := apiclient.DoList[MeetingSchedule](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/meeting-schedules", nil)
	if err != nil {
		return fmt.Errorf("미팅 스케줄 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, schedules)
	}

	headers := []string{"ID", "TITLE", "CRON", "RECURRENCE"}
	rows := make([][]string, len(schedules))
	for i, s := range schedules {
		shortID := s.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{shortID, s.Title, s.CronExpression, s.RecurrenceType}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}
