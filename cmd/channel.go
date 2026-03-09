// channel.go는 채널 관련 CLI 명령어를 구현합니다.
// channel list/show/create/delete/members/config 서브커맨드
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/spf13/cobra"
)

// Channel은 채널 기본 정보를 나타냅니다.
type Channel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	IsArchived  bool   `json:"is_archived"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// ChannelWithUnreadCount는 멤버 수와 읽지 않은 메시지 수를 포함한 채널 정보입니다.
type ChannelWithUnreadCount struct {
	Channel
	MemberCount int `json:"member_count"`
	UnreadCount int `json:"unread_count"`
}

// ChannelMember는 채널 멤버 정보를 나타냅니다.
type ChannelMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// ChannelConfig는 채널 설정 정보를 나타냅니다.
type ChannelConfig struct {
	ID     string                 `json:"id"`
	Config map[string]interface{} `json:"config,omitempty"`
}

var (
	channelJSONOutput bool
	channelListType   string
	channelName       string
	channelDesc       string
)

// channelCmd는 channel 서브커맨드의 루트입니다.
var channelCmd = &cobra.Command{
	Use:   "channel",
	Short: "채널 관련 명령어",
	Long:  `채널 목록 조회, 상세 조회, 생성, 삭제, 멤버 조회, 설정 조회 기능을 제공합니다.`,
}

// channelListCmd는 채널 목록을 조회합니다.
var channelListCmd = &cobra.Command{
	Use:   "list",
	Short: "채널 목록 조회",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		channelType, _ := cmd.Flags().GetString("type")
		client.SetJSONOutput(json)
		return runChannelList(client, os.Stdout, channelType, json)
	},
}

// channelShowCmd는 채널 상세 정보를 조회합니다.
var channelShowCmd = &cobra.Command{
	Use:   "show <channel-id>",
	Short: "채널 상세 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runChannelShow(client, os.Stdout, args[0], json)
	},
}

// channelCreateCmd는 새 채널을 생성합니다.
var channelCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "채널 생성",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("desc")
		client.SetJSONOutput(json)
		return runChannelCreate(client, os.Stdout, name, desc, json)
	},
}

// channelDeleteCmd는 채널을 삭제합니다.
var channelDeleteCmd = &cobra.Command{
	Use:   "delete <channel-id>",
	Short: "채널 삭제",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runChannelDelete(client, os.Stdout, args[0])
	},
}

// channelMembersCmd는 채널 멤버 목록을 조회합니다.
var channelMembersCmd = &cobra.Command{
	Use:   "members <channel-id>",
	Short: "채널 멤버 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		json, _ := cmd.Flags().GetBool("json")
		client.SetJSONOutput(json)
		return runChannelMembers(client, os.Stdout, args[0], json)
	},
}

// channelConfigCmd는 채널 설정을 조회합니다.
var channelConfigCmd = &cobra.Command{
	Use:   "config <channel-id>",
	Short: "채널 설정 조회",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}
		return runChannelConfig(client, os.Stdout, args[0])
	},
}

func init() {
	rootCmd.AddCommand(channelCmd)
	channelCmd.AddCommand(channelListCmd)
	channelCmd.AddCommand(channelShowCmd)
	channelCmd.AddCommand(channelCreateCmd)
	channelCmd.AddCommand(channelDeleteCmd)
	channelCmd.AddCommand(channelMembersCmd)
	channelCmd.AddCommand(channelConfigCmd)

	// --json 플래그를 서브커맨드에 추가
	for _, sub := range []*cobra.Command{channelListCmd, channelShowCmd, channelCreateCmd, channelMembersCmd} {
		sub.Flags().BoolVar(&channelJSONOutput, "json", false, "JSON 형식으로 출력")
	}

	// list 전용 --type 플래그
	channelListCmd.Flags().StringVar(&channelListType, "type", "", "채널 유형 필터 (dm|group|all)")

	// create 전용 플래그
	channelCreateCmd.Flags().StringVar(&channelName, "name", "", "채널 이름 (필수)")
	channelCreateCmd.Flags().StringVar(&channelDesc, "desc", "", "채널 설명")
}

// runChannelList는 채널 목록을 조회하고 출력합니다.
// channelType이 "dm"이면 DM 채널 목록을 조회합니다.
func runChannelList(client *apiclient.Client, out io.Writer, channelType string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	// DM 채널 조회
	if channelType == "dm" {
		dmChannels, err := apiclient.DoList[DMChannel](client, ctx, "GET",
			"/api/v1/workspaces/"+workspaceID+"/dm-channels", nil)
		if err != nil {
			return fmt.Errorf("DM 채널 목록 조회 실패: %w", err)
		}

		if jsonOutput {
			return apiclient.PrintJSON(out, dmChannels)
		}

		headers := []string{"ID", "NAME", "TYPE"}
		rows := make([][]string, len(dmChannels))
		for i, ch := range dmChannels {
			// ID는 첫 8자만 표시
			shortID := ch.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			rows[i] = []string{shortID, ch.Name, ch.Type}
		}
		apiclient.PrintTable(out, headers, rows)
		return nil
	}

	// 일반 채널 조회 (기본/all/group)
	channels, err := apiclient.DoList[ChannelWithUnreadCount](client, ctx, "GET",
		"/api/v1/workspaces/"+workspaceID+"/channels", nil)
	if err != nil {
		return fmt.Errorf("채널 목록 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, channels)
	}

	headers := []string{"ID", "NAME", "TYPE", "MEMBERS", "UNREAD"}
	rows := make([][]string, len(channels))
	for i, ch := range channels {
		// ID는 첫 8자만 표시
		shortID := ch.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		rows[i] = []string{
			shortID,
			ch.Name,
			ch.Type,
			fmt.Sprintf("%d", ch.MemberCount),
			fmt.Sprintf("%d", ch.UnreadCount),
		}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runChannelShow는 채널 상세 정보를 출력합니다.
func runChannelShow(client *apiclient.Client, out io.Writer, channelID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(channelID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	ch, err := apiclient.Do[Channel](client, ctx, "GET", "/api/v1/channels/"+channelID, nil)
	if err != nil {
		return fmt.Errorf("채널 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, ch)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: ch.ID},
		{Key: "Name", Value: ch.Name},
		{Key: "Type", Value: ch.Type},
		{Key: "Description", Value: ch.Description},
		{Key: "IsArchived", Value: fmt.Sprintf("%v", ch.IsArchived)},
		{Key: "CreatedAt", Value: ch.CreatedAt},
	})
	return nil
}

// runChannelCreate는 새 채널을 생성합니다.
func runChannelCreate(client *apiclient.Client, out io.Writer, name, desc string, jsonOutput bool) error {
	workspaceID := client.WorkspaceID()
	body := map[string]string{"name": name, "description": desc}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	ch, err := apiclient.Do[Channel](client, ctx, "POST",
		"/api/v1/workspaces/"+workspaceID+"/channels", body)
	if err != nil {
		return fmt.Errorf("채널 생성 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, ch)
	}

	apiclient.PrintDetail(out, []apiclient.KeyValue{
		{Key: "ID", Value: ch.ID},
		{Key: "Name", Value: ch.Name},
		{Key: "Type", Value: ch.Type},
	})
	return nil
}

// runChannelDelete는 채널을 삭제합니다.
func runChannelDelete(client *apiclient.Client, out io.Writer, channelID string) error {
	if err := apiclient.ValidateID(channelID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	_, err := apiclient.Do[map[string]interface{}](client, ctx, "DELETE",
		"/api/v1/channels/"+channelID, nil)
	if err != nil {
		return fmt.Errorf("채널 삭제 실패: %w", err)
	}

	fmt.Fprintf(out, "채널 삭제 완료: %s\n", channelID)
	return nil
}

// runChannelMembers는 채널 멤버 목록을 출력합니다.
func runChannelMembers(client *apiclient.Client, out io.Writer, channelID string, jsonOutput bool) error {
	if err := apiclient.ValidateID(channelID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	members, err := apiclient.DoList[ChannelMember](client, ctx, "GET",
		"/api/v1/channels/"+channelID+"/members", nil)
	if err != nil {
		return fmt.Errorf("채널 멤버 조회 실패: %w", err)
	}

	if jsonOutput {
		return apiclient.PrintJSON(out, members)
	}

	headers := []string{"ID", "NAME", "ROLE"}
	rows := make([][]string, len(members))
	for i, m := range members {
		rows[i] = []string{m.ID, m.Name, m.Role}
	}
	apiclient.PrintTable(out, headers, rows)
	return nil
}

// runChannelConfig는 채널 설정을 JSON 형식으로 출력합니다.
// config는 동적 구조이므로 항상 JSON으로 출력합니다.
func runChannelConfig(client *apiclient.Client, out io.Writer, channelID string) error {
	if err := apiclient.ValidateID(channelID); err != nil {
		return err
	}

	ctx, cancel := apiclient.NewContextWithTimeout(apiclient.DefaultAPITimeout)
	defer cancel()

	config, err := apiclient.Do[ChannelConfig](client, ctx, "GET",
		"/api/v1/channels/"+channelID+"/config", nil)
	if err != nil {
		return fmt.Errorf("채널 설정 조회 실패: %w", err)
	}

	return apiclient.PrintJSON(out, config)
}
