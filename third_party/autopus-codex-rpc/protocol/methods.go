package protocol

// --- JSON-RPC 메서드 상수 (Bridge 스타일: MethodXxx 네이밍) ---

const (
	// MethodInitialize는 초기화 요청 메서드이다.
	MethodInitialize = "initialize"
	// MethodInitialized는 초기화 완료 알림 메서드이다.
	MethodInitialized = "initialized"

	// MethodAccountLoginStart는 인증 시작 메서드이다.
	MethodAccountLoginStart = "account/login/start"

	// MethodThreadStart는 Thread 생성 메서드이다.
	MethodThreadStart = "thread/start"
	// MethodThreadResume는 Thread 재개 메서드이다.
	MethodThreadResume = "thread/resume"

	// MethodTurnStart는 Turn 시작 메서드이다.
	MethodTurnStart = "turn/start"
	// MethodTurnInterrupt는 Turn 중단 메서드이다.
	MethodTurnInterrupt = "turn/interrupt"

	// MethodTurnCompleted는 Turn 완료 알림 메서드이다 (서버 -> 클라이언트).
	MethodTurnCompleted = "turn/completed"
	// MethodItemStarted는 Item 시작 알림 메서드이다 (서버 -> 클라이언트).
	MethodItemStarted = "item/started"
	// MethodItemCompleted는 Item 완료 알림 메서드이다 (서버 -> 클라이언트).
	MethodItemCompleted = "item/completed"

	// MethodAgentMessageDelta는 에이전트 메시지 증분 알림 메서드이다.
	MethodAgentMessageDelta = "item/agentMessage/delta"
	// MethodCommandExecutionOutputDelta는 명령 실행 출력 증분 알림 메서드이다.
	MethodCommandExecutionOutputDelta = "item/commandExecution/outputDelta"
	// MethodFileChangeOutputDelta는 파일 변경 출력 증분 알림 메서드이다.
	MethodFileChangeOutputDelta = "item/fileChange/outputDelta"

	// MethodCommandExecutionApproval은 명령 실행 승인 요청 알림 메서드이다.
	MethodCommandExecutionApproval = "item/commandExecution/requestApproval"
	// MethodFileChangeApproval은 파일 변경 승인 요청 알림 메서드이다.
	MethodFileChangeApproval = "item/fileChange/requestApproval"

	// MethodToolCall은 서버가 클라이언트에게 동적 도구 실행을 요청할 때 사용하는 메서드이다 (서버 -> 클라이언트 요청).
	MethodToolCall = "item/tool/call"

	// --- Codex v0.114.0 새 이벤트 메서드 (codex/event/* 네임스페이스) ---

	// MethodCodexAgentMessageDelta는 에이전트 메시지 증분 이벤트이다 (v0.114.0+).
	// item/agentMessage/delta의 새 이름.
	MethodCodexAgentMessageDelta = "codex/event/agent_message_delta"

	// MethodCodexAgentMessageContentDelta는 에이전트 메시지 콘텐츠 증분 이벤트이다 (v0.114.0+).
	// delta 또는 content 필드에 텍스트가 담겨 온다.
	MethodCodexAgentMessageContentDelta = "codex/event/agent_message_content_delta"

	// MethodCodexAgentMessage는 완성된 에이전트 메시지 이벤트이다 (v0.114.0+).
	// 모든 델타가 도착한 뒤 전체 텍스트를 포함하여 발행된다.
	MethodCodexAgentMessage = "codex/event/agent_message"

	// MethodCodexItemCompleted는 아이템 완료 이벤트이다 (v0.114.0+).
	// item/completed의 새 이름.
	MethodCodexItemCompleted = "codex/event/item_completed"

	// MethodCodexItemStarted는 아이템 시작 이벤트이다 (v0.114.0+).
	// item/started의 새 이름.
	MethodCodexItemStarted = "codex/event/item_started"

	// MethodCodexTaskComplete는 Turn(태스크) 완료 이벤트이다 (v0.114.0+).
	// turn/completed의 새 이름.
	MethodCodexTaskComplete = "codex/event/task_complete"

	// --- REQ-006: Turn 조종 메서드 ---

	// MethodTurnSteer는 진행 중인 Turn의 방향을 전환하는 메서드이다.
	MethodTurnSteer = "turn/steer"

	// --- REQ-009: Thread 관리 메서드 ---

	// MethodThreadFork는 Thread를 포크하는 메서드이다.
	MethodThreadFork = "thread/fork"
	// MethodThreadRead는 Thread 정보를 읽는 메서드이다.
	MethodThreadRead = "thread/read"
	// MethodThreadList는 Thread 목록을 조회하는 메서드이다.
	MethodThreadList = "thread/list"
	// MethodThreadRollback는 Thread를 특정 Turn으로 롤백하는 메서드이다.
	MethodThreadRollback = "thread/rollback"

	// --- REQ-011: Review 메서드 ---

	// MethodReviewStart는 코드 리뷰를 시작하는 메서드이다.
	MethodReviewStart = "review/start"

	// --- REQ-016: Model 목록 메서드 ---

	// MethodModelList는 사용 가능한 모델 목록을 조회하는 메서드이다.
	MethodModelList = "model/list"

	// --- REQ-012: Config 메서드 ---

	// MethodConfigRead는 현재 설정을 읽는 메서드이다.
	MethodConfigRead = "config/read"
	// MethodConfigValueWrite는 단일 설정 값을 쓰는 메서드이다.
	MethodConfigValueWrite = "config/value/write"
	// MethodConfigBatchWrite는 여러 설정 값을 일괄 쓰는 메서드이다.
	MethodConfigBatchWrite = "config/batchWrite"

	// MethodSkillsList는 사용 가능한 스킬 목록을 조회하는 메서드이다.
	MethodSkillsList = "skills/list"
	// MethodMcpServerStatusList는 MCP 서버 상태 목록을 조회하는 메서드이다.
	MethodMcpServerStatusList = "mcpServerStatus/list"
	// MethodExperimentalFeatureList는 실험적 기능 목록을 조회하는 메서드이다.
	MethodExperimentalFeatureList = "experimentalFeature/list"
)

// --- REQ-010: 알림 메서드 상수 ---

const (
	// MethodTurnPlanUpdated는 Turn 플랜 업데이트 알림 메서드이다 (서버 -> 클라이언트).
	MethodTurnPlanUpdated = "turn/plan/updated"
	// MethodTurnDiffUpdated는 Turn diff 업데이트 알림 메서드이다 (서버 -> 클라이언트).
	MethodTurnDiffUpdated = "turn/diff/updated"
	// MethodItemReasoningSummaryTextDelta는 추론 요약 텍스트 증분 알림 메서드이다.
	MethodItemReasoningSummaryTextDelta = "item/reasoning/summaryTextDelta"
	// MethodItemReasoningTextDelta는 추론 텍스트 증분 알림 메서드이다.
	MethodItemReasoningTextDelta = "item/reasoning/textDelta"
	// MethodThreadStatusChanged는 Thread 상태 변경 알림 메서드이다.
	MethodThreadStatusChanged = "thread/status/changed"
	// MethodThreadTokenUsageUpdated는 Thread 토큰 사용량 업데이트 알림 메서드이다.
	MethodThreadTokenUsageUpdated = "thread/tokenUsage/updated"
)

// --- REQ-010: 아이템 타입 상수 ---

const (
	// ItemTypeReasoning은 추론 아이템 타입이다.
	ItemTypeReasoning = "reasoning"
	// ItemTypePlan은 실행 계획 아이템 타입이다.
	ItemTypePlan = "plan"
	// ItemTypeContextCompaction은 컨텍스트 압축 아이템 타입이다.
	ItemTypeContextCompaction = "contextCompaction"
	// ItemTypeWebSearch는 웹 검색 아이템 타입이다.
	ItemTypeWebSearch = "webSearch"
	// ItemTypeImageView는 이미지 뷰 아이템 타입이다.
	ItemTypeImageView = "imageView"
)
